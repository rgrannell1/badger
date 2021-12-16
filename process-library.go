package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

/*
 * Make directories for each cluster
 */
func (clust *MediaCluster) MakeClusterDirs(dst string) error {
	for idx := range clust.entries {
		cluster_dir := filepath.Join(dst, fmt.Sprint(idx))
		err := os.MkdirAll(cluster_dir, os.ModePerm)

		if err != nil {
			return err
		}
	}

	return nil
}

func MakeFolders(to string, clusters int) error {
	for idx := 0; idx < clusters; idx++ {
		cluster_dir := filepath.Join(to, fmt.Sprint(idx))
		err := os.MkdirAll(cluster_dir, os.ModePerm)

		if err != nil {
			return err
		}
	}

	return nil
}

type BlurStore struct {
	data sync.Map
}

func (store *BlurStore) SaveBlur(media *Media) (float64, error) {
	prefix := media.GetPrefix()
	blur, err := media.GetBlur()

	if err != nil {
		return 0, err
	}

	store.data.Store(prefix, blur)
	return blur, nil
}

func (store *BlurStore) GetStoredBlur(media *Media) float64 {
	prefix := media.GetPrefix()
	val, ok := store.data.Load(prefix)

	if !ok {
		return -1
	} else {
		return val.(float64)
	}
}

/*
 * Copy files and emit error|media sumtypes to the output channel
 */
func CopyFiles(procCount int, copyChan chan Either[Media]) chan Either[Media] {
	results := make(chan Either[Media], procCount)

	// start several goroutines that write to results
	for pid := 0; pid < procCount; pid++ {
		go func() {
			// enumerate over copy-chan; first to grab will win
			for pair := range copyChan {
				media := pair.Media
				err := pair.Error

				// pipeline any existing errors
				if err != nil {
					results <- Either[Media]{media, err}
					continue
				}

				// does the file exist?
				sourceFileStat, err := os.Stat(media.source)
				if err != nil {
					results <- Either[Media]{media, err}
					continue
				}

				// is it a plain old file?
				if !sourceFileStat.Mode().IsRegular() {
					err := errors.New(media.source + " is not a regular file")
					results <- Either[Media]{media, err}
					continue
				}

				// open the media source
				source, err := os.Open(media.source)
				if err != nil {
					results <- Either[Media]{media, err}
					return
				}

				// blur will be present in pipeline
				blurPath := media.GetChosenName(float64(media.blur))

				// if the destination exists, continue and update bar
				if _, err := os.Stat(blurPath); errors.Is(err, os.ErrNotExist) {
					results <- Either[Media]{media, nil}
					continue
				}

				dest, err := os.Create(blurPath)

				if err != nil {
					results <- Either[Media]{media, err}
				}

				// does not exist' copy from source to destination file
				_, err = io.Copy(dest, source)

				if err != nil {
					results <- Either[Media]{media, err}
					continue
				}

				// copied; close the source
				err = source.Close()

				if err != nil {
					results <- Either[Media]{media, err}
					continue
				}

				// copied; close the destination file
				err = dest.Close()

				if err != nil {
					results <- Either[Media]{media, err}
					continue
				}

				// all good!
				results <- Either[Media]{media, nil}
			}

		}()
	}

	return results
}

/*
 * Calculate the blur for each image, and start copy-jobs afterwards
 */
func CalcuateBlur(procCount int, library *MediaList, clusters *MediaCluster) chan Either[Media] {
	results := make(chan Either[Media], len(clusters.entries))

	// a local channel, to distibute media input over
	mediaChan := make(chan Media, len(clusters.entries))
	defer close(mediaChan)

	for pid := 0; pid < procCount; pid++ {
		go func(pid int) {
			for media := range mediaChan {
				mediaType := media.GetType()

				// just copy these as-is, without updating blur-value
				if mediaType == UNKNOWN || mediaType == VIDEO {
					results <- Either[Media]{media, nil}
					continue
				}

				// assume all raw files have a corresponding jpeg
				// for the moment, so skip non-photos

				if mediaType != PHOTO {
					continue
				}

				// get & set blur
				blur, err := media.GetBlur()
				if err != nil {
					results <- Either[Media]{media, err}
					continue
				}
				media.blur = int(blur)

				// look up files with the same prefix, copy blur and prefix
				for _, shared := range library.GetByPrefix(&media) {
					shared.clusterId = media.clusterId
					shared.blur = int(blur)

					results <- Either[Media]{*shared, nil}
				}
			}
		}(pid)
	}

	for _, media := range clusters.entries {
		mediaChan <- media
	}

	return results
}

type Either[T any] struct {
	Media T
	Error error
}

/*
 * Compute blur, and copy files across
 */
func ProcessLibrary(opts *BadgerOpts, clusters *MediaCluster, facts *Facts, library *MediaList) error {
	// construct folders for each cluster, and the root folder
	err := MakeFolders(opts.to, clusters.clusters)
	if err != nil {
		return err
	}

	COPY_PROCS := 10
	BLUR_PROCS := runtime.NumCPU() - 1

	bar := NewProgressBar(int64(facts.Size), facts)
	copyJobs := make(chan Either[Media], len(clusters.entries))
	defer close(copyJobs)

	// iterate over media, and either write directly to copyjobs (video, etc) or calculate blur and then
	// write to blur-jobs. Start this before starting copy-job so it's set up to receive
	go func() {
		for blurRes := range CalcuateBlur(BLUR_PROCS, library, clusters) {
			copyJobs <- blurRes
		}
	}()

	// range over copied file results
	for copyRes := range CopyFiles(COPY_PROCS, copyJobs) {
		if copyRes.Error != nil {
			return copyRes.Error
		} else {
			bar.Update(&copyRes.Media)
		}
	}

	return nil
}
