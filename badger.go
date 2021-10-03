package main

import (
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"strings"

	"bitbucket.org/sjbog/go-dbscan"
	ed "github.com/Ernyoke/Imger/edgedetection"
	"github.com/Ernyoke/Imger/imgio"
	"github.com/Ernyoke/Imger/padding"
	"github.com/docopt/docopt-go"
	"github.com/manifoldco/promptui"
	"github.com/rwcarlsen/goexif/exif"
)

func GetCaptureTime(file string) int {
	ctime, err := GetExifCreateTime(file)

	if err != nil {
		return GetMtime(file)
	} else {
		return ctime
	}
}

func GetExifCreateTime(file string) (int, error) {
	conn, err := os.Open(file)
	if err != nil {
		return 0, err
	}

	metaData, err := exif.Decode(conn)
	if err != nil {
		return 0, err
	}

	time, err := metaData.DateTime()

	if err != nil {
		return 0, err
	} else {
		return int(time.Unix()), nil
	}
}

func GetMtime(file string) int {
	stat, err := os.Stat(file)

	if err != nil {
		return 1
	}

	return int(stat.ModTime().Unix())
}

func listMedia(glob string) ([]Media, error) {
	files, err := filepath.Glob(glob)

	if err != nil {
		return nil, err
	}

	if len(files) == 0 {
		return nil, errors.New("badger: the '--from' glob you provided didn't match any files; is your device connected, and the glob valid and not just a directory path?")
	}

	media := make([]Media, len(files))

	for idx, file := range files {
		media[idx] = Media{
			idx:   fmt.Sprint(idx),
			fpath: file,
			mtime: GetCaptureTime(file),
		}
	}

	return media, nil
}

func clusterMedia(mediaList []Media) MediaCluster {
	var clusterer = dbscan.NewDBSCANClusterer(9, 2)
	clusterer.AutoSelectDimension = false
	clusterer.SortDimensionIndex = 0

	var data = make([]dbscan.ClusterablePoint, len(mediaList))
	var mediaDict = make(map[string]Media)

	for idx, media := range mediaList {
		mediaDict[media.fpath] = media

		data[idx] = &dbscan.NamedPoint{
			Name:  media.fpath,
			Point: []float64{float64(media.mtime)},
		}
	}

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

func PromptCopy(cluster MediaCluster) (bool, error) {
	message := "Would you like to copy " + fmt.Sprint(cluster.MediaCount()) + " media-items to " + fmt.Sprint(cluster.ClusterCount()) + " clusters?"
	prompt := promptui.Select{
		Label: message,
		Items: []string{"yes", "no"},
	}

	_, result, err := prompt.Run()
	if err != nil {
		return false, errors.New(fmt.Sprintf("badger: failed to read user prompt: %v", err))
	}

	if result == "yes" {
		return true, nil
	}

	return false, nil
}

func ComputeBlur(src string) (float64, error) {
	img, err := imgio.ImreadGray(src)

	if err != nil {
		panic(err)
	}

	laplacian, err := ed.LaplacianGray(img, padding.BorderConstant, ed.K4)
	if err != nil {
		return 0, err
	}

	pixSum := 0.0
	for _, pix := range laplacian.Pix {
		pixSum += float64(pix)
	}

	mean := pixSum / float64(len(laplacian.Pix))

	diffs := make([]float64, len(laplacian.Pix))

	for idx, pix := range laplacian.Pix {
		diffs[idx] = math.Pow(float64(pix)-mean, 2)
	}

	variance := 0.0
	for _, diff := range diffs {
		variance += float64(diff)
	}

	variance = variance / float64(len(laplacian.Pix))

	return math.Ceil(variance * 10), nil
}

func IsImage(fpath string) bool {
	return strings.HasSuffix(strings.ToLower(fpath), "jpg") || strings.HasSuffix(strings.ToLower(fpath), "jpeg") || strings.HasSuffix(strings.ToLower(fpath), "png")
}

func FileCopier(copyChan chan []string) {
	for {
		toCopy := <-copyChan

		src := toCopy[0]

		if IsImage(src) {
			blur, err := ComputeBlur(toCopy[0])

			if err != nil {
				fmt.Println("error! ")
			}

			fmt.Println(blur)
		}

	}
}

type MediaCluster struct {
	entries [][]Media
}

func (clust *MediaCluster) MediaCount() int {
	idx := 0

	for _, cluster := range clust.entries {
		idx += len(cluster)
	}

	return idx
}

func (clust *MediaCluster) ClusterCount() int {
	return len(clust.entries)
}

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

func (clust *MediaCluster) ListTargetFiles(dst string) [][]string {
	targets := [][]string{}

	for idx, cluster := range clust.entries {
		cluster_dir := filepath.Join(dst, fmt.Sprint(idx))

		for _, media := range cluster {
			name := media.idx + strings.ToLower(filepath.Ext(media.fpath))
			target := filepath.Join(cluster_dir, name)

			_, err := os.Stat(target)

			if err != nil {
				targets = append(targets, []string{media.fpath, target})
			}
		}
	}

	return targets
}

func BadgerCopy(args BadgerCopyArgs) int {
	mediaList, err := listMedia(args.SrcDir)

	if err != nil {
		fmt.Println(err)
		return 1
	}

	// cluster using DBSCAN
	clusters := clusterMedia(mediaList)

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

	// mostly IO-bound, lets try this!
	PROC_COUNT := runtime.NumCPU() * 2

	copyChans := make([]chan []string, PROC_COUNT)

	for idx := 0; idx < PROC_COUNT; idx++ {
		copyChan := make(chan []string)

		copyChans[idx] = copyChan

		go FileCopier(copyChan)
	}

	// copy media src to target using multiple goroutines.
	for idx, pair := range clusters.ListTargetFiles(args.DstDir) {
		tgtChan := copyChans[idx%PROC_COUNT]
		tgtChan <- pair
	}

	return 0
}

func Badger(opts *docopt.Opts) int {
	if copy, _ := opts.Bool("copy"); copy {
		args := BadgerCopyArgs{}

		srcDir, _ := opts.String("<srcdir>")
		dstDir, _ := opts.String("<destdir>")

		yes, _ := opts.Bool("--yes")

		args.AssumeYes = yes
		args.SrcDir = srcDir
		args.DstDir = dstDir

		f, _ := os.Create("trace.out")
		pprof.StartCPUProfile(f)

		code := BadgerCopy(args)

		f.Close()
		return code
	}

	return 0
}
