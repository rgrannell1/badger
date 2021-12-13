package main

import (
	"errors"
	"fmt"
	"os"

	tm "github.com/buger/goterm"
	"github.com/docopt/docopt-go"
	"github.com/manifoldco/promptui"
	"golang.org/x/sys/unix"
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
}

// Facts about the media-library, like size and count
type Facts struct {
	count        int
	size         int
	videoCount   int
	photoCount   int
	rawCount     int
	unknownCount int
	videoSize    int
	photoSize    int
	rawSize      int
	unknownSize  int
	freeSpace    uint64
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
		count:        library.Size(),
		size:         size,
		videoCount:   videoCount,
		photoCount:   photoCount,
		rawCount:     rawCount,
		unknownCount: unknownCount,
		videoSize:    videoSize,
		photoSize:    photoSize,
		rawSize:      rawSize,
		unknownSize:  unknownSize,
		freeSpace:    freeSpace,
	}, nil
}

/*
 * Ask whether the user wants to proceed with a copy
 */
func PromptCopy(clusters *MediaCluster, facts *Facts, opts *BadgerOpts) (bool, error) {
	if facts.freeSpace < uint64(facts.size) {
		return false, fmt.Errorf("not enough free-space under / to copy files: %v vs %v bytes", facts.freeSpace, facts.size)
	}

	freeAfterMb := (facts.freeSpace - uint64(facts.size)) / 1e9

	message := ("Badger 🦡\n\n" + fmt.Sprint(facts.count) + " media files (" + fmt.Sprint(facts.size/1.0e9) + " gigabytes)\n" +
		fmt.Sprint(facts.photoCount) + " photos (" + fmt.Sprint(facts.photoSize/1.0e9) + " gigabytes)\n" +
		fmt.Sprint(facts.rawCount) + " raw images (" + fmt.Sprint(facts.rawSize/1.0e9) + " gigabytes)\n" +
		fmt.Sprint(facts.videoCount) + " videos (" + fmt.Sprint(facts.videoSize/1.0e9) + " gigabytes)\n\n" +
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
	ProcessLibrary(opts, clusters, facts, library)

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
		}

		err = ValidateOpts(&bopts)
		bail(err)

		os.Exit(Badger(&bopts))
	}

	if copy, _ := opts.Bool("copy"); copy {
		os.Exit(1)
	}
}

/*
 * Get free-space in the target hard-drive
 */
func GetFreeSpace() (uint64, error) {
	var stat unix.Statfs_t

	root := "/home/rg"
	err := unix.Statfs(root, &stat)
	bail(err)

	return stat.Bavail * uint64(stat.Bsize), nil
}
