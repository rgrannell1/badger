package main

import (
	"fmt"
	"math"
	"os"
	"path"
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
}

type MediaType int

const (
	PHOTO MediaType = iota
	RAW
	VIDEO
	UNKNOWN
)

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

func (media *Media) GetDest() string {
	return ""
}

func (media *Media) GetPrefix() string {
	return ""
}

func (media *Media) GetExt() string {
	return path.Ext(media.source)
}

func (media *Media) GetChosenName(blur float64) string {
	return media.dstDir + "/" + fmt.Sprint(media.clusterId) + "/" + fmt.Sprint(blur) + "_" + fmt.Sprint(media.id) + media.GetExt()
}

func (media *Media) GetBlur() (float64, error) {
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
	stat, err := os.Stat(media.source)

	if media.mtime > 0 {
		return media.mtime
	}

	if err != nil {
		return 1
	}

	media.mtime = int(stat.ModTime().Unix())

	return media.mtime
}

func (media *Media) CopyJob() CopyJob {
	if len(media.dstDir) == 0 {
		panic("invalid media; dstDir was missing")
	}

	to := media.dstDir + "/" + fmt.Sprint(media.clusterId) + media.GetExt()

	return CopyJob{
		from: *media,
		to:   to,
	}
}

func (media *Media) GetExifCreateTime() (int, error) {
	conn, err := os.Open(media.source)
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
