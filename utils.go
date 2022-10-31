package main

import (
	"crypto/md5"
	"encoding/hex"
	"io"
	"os"

	"golang.org/x/sys/unix"
)

// Bundles a value error pair
type Either[T any] struct {
	Value T
	Error error
}

/*
 * Get free-space in the target hard-drive
 */
func GetFreeSpace(fpath string) (uint64, error) {
	var stat unix.Statfs_t

	err := unix.Statfs(fpath, &stat)
	bail(err)

	return stat.Bavail * uint64(stat.Bsize), nil
}

/*
 * Hash a file
 *
 */
func GetHash(fpath string) (string, error) {
	file, err := os.Open(fpath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	hashSum := hex.EncodeToString(hash.Sum(nil))

	return hashSum, nil
}
