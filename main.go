package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/docopt/docopt-go"
	"github.com/manifoldco/promptui"
	"golang.org/x/sys/unix"
)

const Usage = `badger: cluster photos by date, and sort by blurriness.

Usage:
	badger --from=<srcdir> --to=<dstdir> [-s <num>|--max-seconds-diff <num>] [-m <num>|--min-points <num>] [-y|--yes]

Description:
	Badger is a tool to cluster and grade large photo-libraries.

Options:
	--max-seconds-diff <num>   [default: 9]
	--min-points <num>         [default: 2]
`

// Badger docopt-arguments
type BadgerOpts struct {
	from           string
	to             string
	maxSecondsDiff float64
	minPoints      int
	yes            bool
}

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

	for _, media := range library.Values() {
		mediaSize, err := media.Size()

		if err != nil {
			return nil, err
		}

		size += int(mediaSize)

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
 *
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

	if !proceed {
		return 0
	}

	ProcessLibrary(opts, clusters, facts)

	// start scoring and copying
	return 0
}

func bail(err error) {
	if err != nil {
		panic(err)
	}
}

func ValidateOpts(opts *BadgerOpts) error {
	if len(opts.from) == 0 {
		return errors.New("--from was length-zero")
	}
	if len(opts.to) == 0 {
		return errors.New("--to was length-zero")
	}

	return nil
}

func main() {
	opts, err := docopt.ParseDoc(Usage)
	bail(err)

	from, err := opts.String("--from")
	bail(err)

	to, err := opts.String("--to")
	bail(err)

	maxSecondsDiff, err := opts.Float64("--max-seconds-diff")
	bail(err)

	yes, _ := opts.Bool("--yes")

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

func GetFreeSpace() (uint64, error) {
	var stat unix.Statfs_t

	root := "/home/rg"
	err := unix.Statfs(root, &stat)
	bail(err)

	return stat.Bavail * uint64(stat.Bsize), nil
}
