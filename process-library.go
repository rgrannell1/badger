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

type CopyJob struct {
	from Media
	to   string
}

type BlurStore struct {
	data sync.Map
}

func (store *BlurStore) SaveBlur(media *Media) {
	prefix := media.GetPrefix()
	store.data.Store(prefix, media.GetBlur())
}

func (store *BlurStore) GetBlur(media *Media) int {
	prefix := media.GetPrefix()
	val, ok := store.data.Load(prefix)

	if !ok {
		return -1
	} else {
		return val.(int)
	}
}

type JobResult struct {
	media Media
	error error
}

/*
 *
 */
func CopyFiles(wg sync.WaitGroup, imageBlur *BlurStore, copyChan chan CopyJob, resultChan chan JobResult, bar *ProgressBar) {
	for job := range copyChan {
		// copy the file, and apply the blur name if possible
		sourceFileStat, err := os.Stat(job.from.source)
		if err != nil {
			resultChan <- JobResult{job.from, err}
			return
		}

		if !sourceFileStat.Mode().IsRegular() {
			err := errors.New(job.from.source + " is not a regular file")
			resultChan <- JobResult{job.from, err}
		}

		source, err := os.Open(job.from.source)
		if err != nil {
			resultChan <- JobResult{job.from, err}
			return
		}

		// retrieve the blur. This should be set prior to copy-job creation by a blur job.
		// it will not be present for videos
		blur := imageBlur.GetBlur(&job.from)
		blurPath := job.from.GetChosenName(blur)

		destination, err := os.Create(blurPath)
		if err != nil {
			resultChan <- JobResult{job.from, err}
			return
		}

		size, err := job.from.Size()
		if err != nil {
			panic(err)
		}

		// if the destination file exists, continue
		if _, err := os.Stat(blurPath); errors.Is(err, os.ErrNotExist) {
			bar.Update(size)
			continue
		}

		_, err = io.Copy(destination, source)

		if err != nil {
			resultChan <- JobResult{job.from, err}
			return
		}

		err = source.Close()

		if err != nil {
			resultChan <- JobResult{job.from, err}
			return
		}

		err = destination.Close()

		if err != nil {
			resultChan <- JobResult{job.from, err}
			return
		}

		bar.Update(size)
	}

	wg.Done()
}

func CalcuateBlur(wg sync.WaitGroup, imageBlur *BlurStore, blurChan chan *Media, bar *ProgressBar) {
	for media := range blurChan {
		imageBlur.SaveBlur(media)
		size, err := media.Size()

		if err != nil {
			panic(err)
		}

		bar.Update(size)
	}

	wg.Done()
}

/*
 * Compute blur, and copy files across
 */
func ProcessLibrary(opts *BadgerOpts, clusters *MediaCluster, facts *Facts) {
	// construct folders for each cluster, and the root folder
	err := MakeFolders(opts.to, clusters.clusters)
	bail(err)

	copyJobs := make(chan CopyJob, len(clusters.entries))
	blurJobs := make(chan *Media, len(clusters.entries))
	resultChan := make(chan JobResult, len(clusters.entries))

	var wg sync.WaitGroup
	var imageBlur BlurStore

	bar := NewProgressBar(int64(facts.size))

	const COPY_COUNT = 10
	wg.Add(COPY_COUNT)

	for copyId := 0; copyId < COPY_COUNT; copyId++ {
		go CopyFiles(wg, &imageBlur, copyJobs, resultChan, bar)
	}

	CPU_COUNT := runtime.NumCPU()
	wg.Add(CPU_COUNT - 1)

	for blurId := 0; blurId < CPU_COUNT-1; blurId++ {
		go CalcuateBlur(wg, &imageBlur, blurJobs, bar)
	}

	// start blur jobs
	for _, media := range clusters.entries {
		mediaType := media.GetType()

		// send this media immediately
		if mediaType == PHOTO {
			blurJobs <- &media
		} else {
			copyJobs <- media.CopyJob()
		}
	}

	select {
	case result := <-resultChan:
		if result.error != nil {
			fmt.Println(result.media)
			panic(result.error)
		}
	default:
	}

	wg.Wait()
}
