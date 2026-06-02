//go:build !windows

package main

import "os"

func hasReparsePoint(info os.FileInfo) bool {
	return false
}

func hasReparsePointPath(path string) bool {
	return false
}
