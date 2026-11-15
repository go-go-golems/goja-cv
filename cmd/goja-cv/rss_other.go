//go:build !unix

package main

// peakRSSBytes is unknown on this platform.
func peakRSSBytes() int64 { return 0 }
