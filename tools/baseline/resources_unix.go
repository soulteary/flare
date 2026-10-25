//go:build darwin || linux

package main

import (
	"runtime"
	"syscall"
)

func processResources() (user, system float64, peakRSS uint64) {
	var usage syscall.Rusage
	if syscall.Getrusage(syscall.RUSAGE_SELF, &usage) != nil {
		return
	}
	user = float64(usage.Utime.Sec) + float64(usage.Utime.Usec)/1e6
	system = float64(usage.Stime.Sec) + float64(usage.Stime.Usec)/1e6
	peakRSS = uint64(usage.Maxrss)
	if runtime.GOOS == "linux" {
		peakRSS *= 1024
	}
	return
}
