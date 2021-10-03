package main

import (
	"math"
	"os"
	"strings"

	ed "github.com/Ernyoke/Imger/edgedetection"
	"github.com/Ernyoke/Imger/imgio"
	"github.com/Ernyoke/Imger/padding"
	"github.com/rwcarlsen/goexif/exif"
)

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
