//go:build unix

package main

import (
	"runtime"
	"syscall"
)

// peakRSSBytes is the process's peak resident set size so far (0 if unknown).
func peakRSSBytes() int64 {
	var ru syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &ru); err != nil {
		return 0
	}
	if runtime.GOOS == "darwin" {
		return ru.Maxrss // bytes on macOS
	}
	return ru.Maxrss * 1024 // kilobytes on Linux and the BSDs
}
