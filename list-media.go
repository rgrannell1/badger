package main

import (
	"errors"
	"path/filepath"
)

type MediaList struct {
	library []*Media
}

func (media *MediaList) Values() []*Media {
	return media.library
}

func (media *MediaList) Size() int {
	return len(media.library)
}

func (library *MediaList) GetByPrefix(prefix string) []*Media {
	return ""
}

func NewMediaList(library []*Media) *MediaList {
	return &MediaList{library}
}

func (opts *BadgerOpts) ListMedia() (*MediaList, error) {
	files, err := filepath.Glob(opts.from)

	// double-check listed files
	if err != nil {
		return NewMediaList([]*Media{}), err
	}

	if len(files) == 0 {
		return NewMediaList([]*Media{}), errors.New("badger: the '--from' glob you provided didn't match any files; is your device connected, and the glob valid and not just a directory path?")
	}

	if len(files) == 1 {
		return NewMediaList([]*Media{}), errors.New("badger: the '--from' glob only matched one file; is your device connected, and the glob valid and not just a directory path?")
	}

	// construct media objects for each file
	library := make([]*Media, len(files))

	for idx, fpath := range files {
		media := Media{
			source: fpath,
			dstDir: opts.to,
			id:     idx,
		}

		library[idx] = &media
	}

	return NewMediaList(library), nil
}
