//go:build windows

package main

import (
	"os"
	"syscall"
	"time"
)

func getCreatedUTC(info os.FileInfo) *time.Time {
	data, ok := info.Sys().(*syscall.Win32FileAttributeData)
	if !ok {
		return nil
	}

	t := time.Unix(0, data.CreationTime.Nanoseconds()).UTC()
	return &t
}
