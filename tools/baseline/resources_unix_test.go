//go:build darwin || linux

package main

import (
	"math"
	"testing"
)

func TestMaxRSSBytes(t *testing.T) {
	const maxLinuxSample = int64(math.MaxUint64 / 1024)
	for _, test := range []struct {
		name         string
		maxRSS       int64
		bytesPerUnit uint64
		want         uint64
	}{
		{name: "Darwin bytes", maxRSS: 12345, bytesPerUnit: 1, want: 12345},
		{name: "Linux KiB", maxRSS: 12345, bytesPerUnit: 1024, want: 12641280},
		{name: "zero bytes", maxRSS: 0, bytesPerUnit: 1},
		{name: "zero KiB", maxRSS: 0, bytesPerUnit: 1024},
		{name: "negative bytes", maxRSS: -1, bytesPerUnit: 1},
		{name: "negative KiB", maxRSS: -1, bytesPerUnit: 1024},
		{name: "minimum signed sample", maxRSS: math.MinInt64, bytesPerUnit: 1024},
		{name: "maximum byte sample", maxRSS: math.MaxInt64, bytesPerUnit: 1, want: math.MaxInt64},
		{name: "largest representable KiB sample", maxRSS: maxLinuxSample, bytesPerUnit: 1024, want: math.MaxUint64 - 1023},
		{name: "KiB overflow boundary", maxRSS: maxLinuxSample + 1, bytesPerUnit: 1024},
		{name: "maximum signed KiB sample", maxRSS: math.MaxInt64, bytesPerUnit: 1024},
		{name: "invalid zero unit", maxRSS: 12345},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := maxRSSBytes(test.maxRSS, test.bytesPerUnit); got != test.want {
				t.Errorf("maxRSSBytes(%d, %d) = %d, want %d", test.maxRSS, test.bytesPerUnit, got, test.want)
			}
		})
	}
}
