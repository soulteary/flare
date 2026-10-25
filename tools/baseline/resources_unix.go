//go:build darwin || linux

package main

import (
	"math"
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
	bytesPerUnit := uint64(1)
	if runtime.GOOS == "linux" {
		bytesPerUnit = 1024
	}
	peakRSS = maxRSSBytes(int64(usage.Maxrss), bytesPerUnit)
	return
}

// maxRSSBytes converts Darwin's byte and Linux's KiB samples without wrapping.
// Invalid or unrepresentable samples use the same zero value as unavailable data.
func maxRSSBytes(maxRSS int64, bytesPerUnit uint64) uint64 {
	if maxRSS <= 0 || bytesPerUnit == 0 {
		return 0
	}
	rss := uint64(maxRSS)
	if rss > math.MaxUint64/bytesPerUnit {
		return 0
	}
	return rss * bytesPerUnit
}
