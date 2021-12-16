package main

import "golang.org/x/sys/unix"

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
