//go:build darwin

package main

import (
	"os"
	"syscall"
	"time"
)

func getCreatedUTC(info os.FileInfo) *time.Time {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return nil
	}

	ts := stat.Birthtimespec
	if ts.Sec == 0 && ts.Nsec == 0 {
		return nil
	}

	t := time.Unix(ts.Sec, ts.Nsec).UTC()
	return &t
}
