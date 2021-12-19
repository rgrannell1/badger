package main

import (
	"crypto/md5"
	"encoding/hex"
	"io"
	"os"

	"golang.org/x/sys/unix"
)

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
