package main

import (
	"errors"
	"fmt"
	"math"
	"os"
	"path"
	"path/filepath"
	"strings"

	ed "github.com/Ernyoke/Imger/edgedetection"
	"github.com/Ernyoke/Imger/imgio"
	"github.com/Ernyoke/Imger/padding"
	"github.com/rwcarlsen/goexif/exif"
)

type Media struct {
	source    string
	dstDir    string
	blur      int
	size      int64
	mtime     int
	clusterId int
	id        int
	copied    bool
	exifData  *PhotoInformation
	hash      string
}

type MediaType string

const (
	PHOTO   MediaType = "photo"
	RAW               = "raw"
	VIDEO             = "video"
	UNKNOWN           = "unknown"
)

// cache mtime, information about media type
func (media *Media) LoadInformation() error {
	// memoised
	media.GetMtime()
	_, err := media.GetHash()
	if err != nil {
		return err
	}

	_, err = media.GetInformation()
	if err != nil {
		return err
	}

	return nil
}

/*
 * Get the media type based on file-extensions
 */
func (media *Media) GetType() MediaType {
	ext := strings.ToLower(media.GetExt())

	switch ext {
	case ".jpg", ".jpeg", ".png":
		return PHOTO
	case ".rw2", ".raw":
		return RAW
	case ".mp4":
		return VIDEO
	}

	return UNKNOWN
}

func (media *Media) GetPrefix() string {
	return strings.TrimSuffix(media.source, media.GetExt())
}

func (media *Media) GetExt() string {
	return path.Ext(media.source)
}

/*
 * Get the target filename for the copied media
 */
func (media *Media) GetDestinationPath() string {
	blur := media.blur

	name := ""
	root := filepath.Join(media.dstDir, fmt.Sprint(media.clusterId))

	if blur == -1 {
		name = fmt.Sprint(media.id) + media.GetExt()
	} else {
		name = fmt.Sprint(blur) + "_" + fmt.Sprint(media.id) + media.GetExt()
	}

	return filepath.Join(root, name)
}

/*
 * Check whether the destination file exists
 */
func (media *Media) DestinationExists() (bool, error) {
	dest := media.GetDestinationPath()
	_, err := os.Stat(dest)

	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return false, err
		} else {
			return false, nil
		}
	}

	return true, nil
}

func (media *Media) DestinationHash() (string, error) {
	return GetHash(media.GetDestinationPath())
}

func (media *Media) Size() (int64, error) {
	// memoise
	if media.size > 0 {
		return media.size, nil
	}

	fi, err := os.Stat(media.source)
	if err != nil {
		return -1, err
	}

	size := fi.Size()

	media.size = size
	return media.size, nil
}

// Get a media file's mtime
func (media *Media) GetMtime() int {
	if media.mtime > 0 {
		return media.mtime
	}

	stat, err := os.Stat(media.source)

	if err != nil {
		return 1
	}

	media.mtime = int(stat.ModTime().Unix())

	return media.mtime
}

func (media *Media) GetExifCreateTime() (int, error) {
	conn, err := os.Open(media.source)
	defer conn.Close()

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

func (media *Media) GetCreationTime() int {
	ctime, err := media.GetExifCreateTime()

	if err != nil {
		return media.GetMtime()
	} else {
		return ctime
	}
}

type PhotoInformation struct {
	Iso          string
	Aperture     string
	ShutterSpeed string
}

func (media *Media) GetInformation() (*PhotoInformation, error) {
	if media.exifData != nil {
		return media.exifData, nil
	}

	if media.GetType() != PHOTO {
		return &PhotoInformation{}, nil
	}

	conn, err := os.Open(media.source)
	defer conn.Close()

	if err != nil {
		return &PhotoInformation{}, err
	}

	metaData, err := exif.Decode(conn)

	if err != nil {
		return &PhotoInformation{}, err
	}

	// attempt to extract and store exif information
	fstop := ""
	iso := ""
	shutter := ""

	fstopTag, err := metaData.Get(exif.FocalLength)
	if err == nil {
		fstop, _ = fstopTag.StringVal()
	}

	isoTag, err := metaData.Get(exif.ISOSpeedRatings)
	if err == nil {
		iso, _ = isoTag.StringVal()
	}

	shutterTag, err := metaData.Get(exif.ShutterSpeedValue)
	if err == nil {
		shutter, _ = shutterTag.StringVal()
	}

	info := PhotoInformation{
		Iso:          iso,
		Aperture:     fstop,
		ShutterSpeed: shutter,
	}

	media.exifData = &info

	return &info, nil
}

/*
 * Get and cache a file hash
 */
func (media *Media) GetHash() (string, error) {
	if len(media.hash) > 0 {
		return media.hash, nil
	}

	hashSum, err := GetHash(media.source)
	if err != nil {
		return "", err
	}

	media.hash = hashSum

	return hashSum, nil
}

func (media *Media) GetBlur() (float64, error) {
	if media.blur > 0 {
		return float64(media.blur), nil
	}

	img, err := imgio.ImreadGray(media.source)

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
