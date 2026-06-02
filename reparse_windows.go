//go:build windows

package main

import (
	"os"
	"syscall"
)

func hasReparsePoint(info os.FileInfo) bool {
	data, ok := info.Sys().(*syscall.Win32FileAttributeData)
	if !ok {
		return false
	}

	return data.FileAttributes&syscall.FILE_ATTRIBUTE_REPARSE_POINT != 0
}

func hasReparsePointPath(path string) bool {
	attrs, err := syscall.GetFileAttributes(syscall.StringToUTF16Ptr(path))
	if err != nil {
		return false
	}

	return attrs&syscall.FILE_ATTRIBUTE_REPARSE_POINT != 0
}
