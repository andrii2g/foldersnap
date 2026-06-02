//go:build !windows && !darwin

package main

import (
	"os"
	"time"
)

func getCreatedUTC(info os.FileInfo) *time.Time {
	return nil
}
