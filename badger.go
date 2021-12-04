package main

import (
	"errors"
	"fmt"

	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"bitbucket.org/sjbog/go-dbscan"
	"github.com/docopt/docopt-go"
	"github.com/manifoldco/promptui"
	"github.com/schollz/progressbar"
)

/*
 * Get the time an image was captured.
 */
func GetCaptureTime(file string) int {
	ctime, err := GetExifCreateTime(file)

	if err != nil {
		return GetMtime(file)
	} else {
		return ctime
	}
}

/*
 * Get the the a file was modified
 */
func GetMtime(file string) int {
	stat, err := os.Stat(file)

	if err != nil {
		return 1
	}

	return int(stat.ModTime().Unix())
}

var fileIdState = 1
var nameIds = map[string]int{}

func FilePrefix(fpath string) string {
	return strings.TrimSuffix(fpath, filepath.Ext(fpath))
}

/*
 * Assign an ID based on a filepath, so raw and jpge images can share properties.
 *
 */
func AssignId(fpath string) string {
	name := FilePrefix(fpath)

	// assign, increment
	if saved, ok := nameIds[name]; ok {
		return fmt.Sprint(saved)
	} else {
		// increment a stateful file id, set in the dictionary
		fileIdState = fileIdState + 1
		nameIds[name] = fileIdState

		return fmt.Sprint(fileIdState)
	}
}

/*
 * List all media in a folder matching a glob.
 */
func listMedia(glob string) ([]Media, error) {
	files, err := filepath.Glob(glob)

	if err != nil {
		return nil, err
	}

	if len(files) == 0 {
		return nil, errors.New("badger: the '--from' glob you provided didn't match any files; is your device connected, and the glob valid and not just a directory path?")
	}

	if len(files) == 1 {
		return nil, errors.New("badger: the '--from' glob only matched one file; is your device connected, and the glob valid and not just a directory path?")
	}

	media := make([]Media, len(files))

	for idx, file := range files {
		photoId := AssignId(file)

		media[idx] = Media{
			idx:   photoId,
			fpath: file,
			mtime: GetCaptureTime(file),
		}
	}

	return media, nil
}

// Use DBSCAN to cluster media files together based on the time
// they were captured
func clusterMedia(eps float64, minPoints int, mediaList []Media) MediaCluster {
	var clusterer = dbscan.NewDBSCANClusterer(eps, minPoints)
	clusterer.AutoSelectDimension = false
	clusterer.SortDimensionIndex = 0

	var data = make([]dbscan.ClusterablePoint, len(mediaList))
	var mediaDict = make(map[string]Media)

	for idx, media := range mediaList {
		mediaDict[media.fpath] = media

		// create a named point, with the file as the name and the mtime as a
		// dimension it is clustered along
		data[idx] = &dbscan.NamedPoint{
			Name:  media.fpath,
			Point: []float64{float64(media.mtime)},
		}
	}

	// cluster, and restructure the data for use later
	clusters := clusterer.Cluster(data)
	mediaClusters := make([][]Media, len(clusters))

	for idx, cluster := range clusters {
		clusterList := make([]Media, len(cluster))

		for jdx, point := range cluster {
			fpath := point.(*dbscan.NamedPoint).Name
			clusterList[jdx] = mediaDict[fpath]
		}

		mediaClusters[idx] = clusterList
	}

	return MediaCluster{
		entries: mediaClusters,
	}
}

// Prompt whether the user wants to proceed with file copying and clustering.
func PromptCopy(cluster MediaCluster) (bool, error) {
	message := "Would you like to copy " + fmt.Sprint(cluster.MediaCount()) + " media-items to " + fmt.Sprint(cluster.ClusterCount()) + " clusters?"
	prompt := promptui.Select{
		Label: message,
		Items: []string{"yes", "no"},
	}

	_, result, err := prompt.Run()
	if err != nil {
		return false, fmt.Errorf("badger: failed to read user prompt: %v", err)
	}

	if result == "yes" {
		return true, nil
	}

	return false, nil
}

// Compute a file name, based on the file path and the
// amount of blur detected
func BlurName(fpath string, blur float64) string {
	if blur == -1 {
		return fpath
	}

	base := filepath.Base(fpath)
	dir := strings.TrimSuffix(fpath, base)

	return filepath.Join(dir, fmt.Sprint(blur)+"_"+base)
}

// Copy a source file to a destination, and name according to blur
// if possible
func CopyFile(src string, dst string, blur float64) error {
	sourceFileStat, err := os.Stat(src)
	if err != nil {
		return err
	}

	if !sourceFileStat.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file", src)
	}

	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer source.Close()

	// Create destination path; use blur in name if present
	destination, err := os.Create(BlurName(dst, blur))
	if err != nil {
		return err
	}

	defer destination.Close()
	_, err = io.Copy(destination, source)

	return err
}

// Receive from a copy channel, compute blur, and
// copy files
func FileCopier(wg *sync.WaitGroup, copyChan chan []string, errChan chan error, bar *progressbar.ProgressBar) {
	for toCopy := range copyChan {
		src := toCopy[0]
		dst := toCopy[1]

		if IsImage(src) {
			blur, err := ComputeBlur(toCopy[0])

			if err != nil {
				panic(err)
			}

			copyErr := CopyFile(src, dst, blur)

			if copyErr != nil {
				errChan <- copyErr
			}

			wg.Done()
			bar.Add(1)
		} else {
			copyErr := CopyFile(src, dst, -1)
			if copyErr != nil {
				errChan <- copyErr
			}

			wg.Done()
			bar.Add(1)
		}
	}
}

type MediaCluster struct {
	entries [][]Media
}

// Count the number of media entries, across clusters
func (clust *MediaCluster) MediaCount() int {
	idx := 0

	for _, cluster := range clust.entries {
		idx += len(cluster)
	}

	return idx
}

// Count the number of clusters present
func (clust *MediaCluster) ClusterCount() int {
	return len(clust.entries)
}

// Create a directory for each cluster
func (clust *MediaCluster) MakeClusterDirs(dst string) error {
	for idx, _ := range clust.entries {
		cluster_dir := filepath.Join(dst, fmt.Sprint(idx))
		err := os.MkdirAll(cluster_dir, os.ModePerm)

		if err != nil {
			return err
		}
	}

	return nil
}

// List target files, and return pairs of file-paths + destination paths
func (clust *MediaCluster) ListTargetFiles(dst string) [][]string {
	targets := [][]string{}

	for idx, cluster := range clust.entries {
		// the cluster directory we are copying to
		cluster_dir := filepath.Join(dst, fmt.Sprint(idx))

		for _, media := range cluster {
			// the name of the file + extension
			name := media.idx + strings.ToLower(filepath.Ext(media.fpath))
			// the target directory
			target := filepath.Join(cluster_dir, name)

			_, err := os.Stat(target)

			if err != nil {
				targets = append(targets, []string{media.fpath, target})
			}
		}
	}

	return targets
}

// Run Badger's copy task
func BadgerCopy(args BadgerCopyArgs) int {
	mediaList, err := listMedia(args.SrcDir)

	if err != nil {
		fmt.Println(err)
		return 1
	}

	// cluster using DBSCAN
	// Use DBSCAN to cluster media files together based on the time
	// they were captured
	clusters := clusterMedia(9, 2, mediaList)

	// prompt whether to copy, interactively
	if !args.AssumeYes {
		ok, err := PromptCopy(clusters)

		if err != nil {
			fmt.Println(err)
			return 1
		}

		if !ok {
			return 0
		}
	}

	// make cluster directories
	err = clusters.MakeClusterDirs(args.DstDir)
	if err != nil {
		fmt.Println(err)
		return 1
	}

	// start multiple copy processes; blur computation will be
	// CPU-bound so this will be CPU-bound overall
	PROC_COUNT := runtime.NumCPU()

	errChan := make(chan error)
	copyChan := make(chan []string)

	tgtFile := clusters.ListTargetFiles(args.DstDir)
	count := len(tgtFile)
	bar := progressbar.Default(int64(count))

	var wg sync.WaitGroup
	wg.Add(len(tgtFile))

	// Distribute file-copies across processes
	for idx := 0; idx < PROC_COUNT; idx++ {
		go FileCopier(&wg, copyChan, errChan, bar)
	}

	for _, job := range tgtFile {
		copyChan <- job
	}

	close(copyChan)
	select {
	case err := <-errChan:
		panic(err)
	default:
	}

	wg.Wait()
	close(errChan)

	return 0
}

// Start badger
func Badger(opts *docopt.Opts) int {
	if copy, _ := opts.Bool("copy"); copy {
		args := BadgerCopyArgs{}

		srcDir, _ := opts.String("<srcdir>")
		dstDir, _ := opts.String("<destdir>")

		yes, _ := opts.Bool("--yes")

		args.AssumeYes = yes
		args.SrcDir = srcDir
		args.DstDir = dstDir

		code := BadgerCopy(args)

		return code
	}

	return 0
}
