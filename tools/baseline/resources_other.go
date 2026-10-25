//go:build !darwin && !linux

package main

func processResources() (user, system float64, peakRSS uint64) { return }
