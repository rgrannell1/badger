package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"runtime"

	tm "github.com/buger/goterm"
	"github.com/docopt/docopt-go"
	"github.com/google/gops/agent"
	"github.com/manifoldco/promptui"
)

const Usage = `badger: cluster photos by date, and sort by blurriness.

Usage:
	badger cluster --from=<srcglob> --to=<dstdir> [-s <num>|--max-seconds-diff <num>] [-m <num>|--min-points <num>] [-y|--yes]
	badger copy --from=<srcglob> --to=<dstdir> [--media (all|photo|video|raw|unknown)] [--max-iso <iso>] [--min-shutter-speed <speed>]
	badger (-h|--help)

Description:
	Badger is a tool to cluster and grade large photo-libraries.

Commans:
	badger cluster                 cluster photos by date, and sort by blurriness.
	badger copy                    copy media matching a set of filters into a target folder.

Options:
	--from=<srcglob>               source glob
	--to=<dstdir>                  target directory
	--yes                          complete copy without manual prompt
	--max-seconds-diff <num>       max seconds photos can be apart in order to cluster them together [default: 9]
	--min-shutter-speed <speed>    minimum shutter speed for images to copy.
	--min-points <num>             minimum number of media to cluster [default: 2]
	--max-iso <iso>                maximum iso for images to copy.

License:
	The MIT License

	Copyright (c) 2021 Róisín Grannell

	Permission is hereby granted, free of charge, to any person obtaining a copy of this software and associated documentation
	files (the "Software"), to deal in the Software without restriction, including without limitation the rights to use, copy,
	modify, merge, publish, distribute, sublicense, and/or sell copies of the Software, and to permit persons to whom the
	Software is furnished to do so, subject to the following conditions:

	The above copyright notice and this permission notice shall be included in all copies or substantial portions of the Software.

	THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES
	OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS
	BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT
	OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
`

// Badger docopt-arguments
type BadgerOpts struct {
	from           string
	to             string
	maxSecondsDiff float64
	minPoints      int
	yes            bool
	copyWorkers    int
	blurWorkers    int
}

// Facts about the media-library, like size and count
type Facts struct {
	Count        int
	Size         int
	VideoCount   int
	PhotoCount   int
	RawCount     int
	UnknownCount int
	VideoSize    int
	PhotoSize    int
	RawSize      int
	UnknownSize  int
	FreeSpace    uint64
}

/*
 * Panic on error
 */
func bail(err error) {
	if err != nil {
		panic(err)
	}
}

/*
 * Gather facts about the job that will be run
 */
func GatherFacts(library *MediaList) (*Facts, error) {
	size := 0
	videoCount := 0
	photoCount := 0
	rawCount := 0
	videoSize := 0
	photoSize := 0
	unknownSize := 0
	rawSize := 0
	unknownCount := 0

	// enumerate through each media entry
	for _, media := range library.Values() {
		mediaSize, err := media.Size()

		if err != nil {
			return nil, err
		}

		size += int(mediaSize)

		// update statistics for each media type
		switch media.GetType() {
		case PHOTO:
			photoCount += 1
			photoSize += int(mediaSize)
		case VIDEO:
			videoCount += 1
			videoSize += int(mediaSize)
		case RAW:
			rawCount += 1
			rawSize += int(mediaSize)
		case UNKNOWN:
			unknownCount += 1
			unknownSize += int(mediaSize)
		}
	}

	freeSpace, err := GetFreeSpace()
	bail(err)

	return &Facts{
		Count:        library.Size(),
		Size:         size,
		VideoCount:   videoCount,
		PhotoCount:   photoCount,
		RawCount:     rawCount,
		UnknownCount: unknownCount,
		VideoSize:    videoSize,
		PhotoSize:    photoSize,
		RawSize:      rawSize,
		UnknownSize:  unknownSize,
		FreeSpace:    freeSpace,
	}, nil
}

/*
 * Ask whether the user wants to proceed with a copy
 */
func PromptCopy(clusters *MediaCluster, facts *Facts, opts *BadgerOpts) (bool, error) {
	if facts.FreeSpace < uint64(facts.Size) {
		return false, fmt.Errorf("not enough free-space under / to copy files: %v vs %v bytes", facts.FreeSpace, facts.Size)
	}

	freeAfterMb := (facts.FreeSpace - uint64(facts.Size)) / 1e9

	message := ("Badger 🦡\n\n" + fmt.Sprint(facts.Count) + " media files (" + fmt.Sprint(facts.Size/1.0e9) + " gigabytes)\n" +
		fmt.Sprint(facts.PhotoCount) + " photos (" + fmt.Sprint(facts.PhotoSize/1.0e9) + " gigabytes)\n" +
		fmt.Sprint(facts.RawCount) + " raw images (" + fmt.Sprint(facts.RawSize/1.0e9) + " gigabytes)\n" +
		fmt.Sprint(facts.VideoCount) + " videos (" + fmt.Sprint(facts.VideoSize/1.0e9) + " gigabytes)\n\n" +
		"Badger will group this media into " + fmt.Sprint(clusters.ClusterSize()) + " cluster-folders.\n" +
		"there will be " + fmt.Sprint(freeAfterMb) + " gigabytes free after copying")

	fmt.Println(message)

	if opts.yes {
		return true, nil
	}

	prompt := promptui.Select{
		Label: "Would you like to proceed?",
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

/*
 * Core application
 */
func Badger(opts *BadgerOpts) int {
	// list everything that will be targeted
	library, err := opts.ListMedia()

	bail(err)

	// gather information about the media to be clustered
	facts, err := GatherFacts(library)
	bail(err)

	// cluster
	clusters := ClusterMedia(opts.maxSecondsDiff, opts.minPoints, library)

	// prompt whether we want to proceed
	proceed, err := PromptCopy(clusters, facts, opts)
	bail(err)

	tm.Clear()

	if !proceed {
		return 0
	}

	// start processing the media library
	err = ProcessLibrary(opts, clusters, facts, library)
	bail(err)

	// start scoring and copying
	return 0
}

/*
 * Validate badger inputs
 */
func ValidateOpts(opts *BadgerOpts) error {
	if len(opts.from) == 0 {
		return errors.New("--from was length-zero")
	}
	if len(opts.to) == 0 {
		return errors.New("--to was length-zero")
	}

	return nil
}

/*
 * Start of the application
 */
func main() {
	if err := agent.Listen(agent.Options{}); err != nil {
		log.Fatal(err)
	}

	opts, err := docopt.ParseDoc(Usage)
	bail(err)

	from, err := opts.String("--from")
	bail(err)

	to, err := opts.String("--to")
	bail(err)

	if cluster, _ := opts.Bool("cluster"); cluster {
		yes, _ := opts.Bool("--yes")

		maxSecondsDiff, err := opts.Float64("--max-seconds-diff")
		bail(err)

		bopts := BadgerOpts{
			from:           from,
			to:             to,
			maxSecondsDiff: maxSecondsDiff,
			yes:            yes,
			copyWorkers:    10,
			blurWorkers:    runtime.NumCPU() - 1,
		}

		err = ValidateOpts(&bopts)
		bail(err)

		os.Exit(Badger(&bopts))
	}

	if copy, _ := opts.Bool("copy"); copy {
		os.Exit(1)
	}
}
