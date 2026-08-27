//go:build darwin

package telemetry

import (
	"fmt"

	"golang.org/x/sys/unix"
)

func totalSystemMemoryMB() (float64, error) {
	bytes, err := unix.SysctlUint32("hw.memsize")
	if err != nil {
		// hw.memsize returns 64-bit on modern Darwin; try the raw sysctl
		raw, err2 := unix.SysctlRaw("hw.memsize")
		if err2 != nil {
			return 0, fmt.Errorf("could not read system memory: %w", err)
		}
		if len(raw) < 8 {
			return 0, fmt.Errorf("could not read system memory: unexpected hw.memsize size %d", len(raw))
		}
		var total uint64
		for i := 0; i < 8; i++ {
			total |= uint64(raw[i]) << (i * 8)
		}
		return float64(total) / oneMb, nil
	}
	return float64(bytes) / oneMb, nil
}

func readCPUTimes() (cpuTimes, error) {
	// No /proc/stat on Darwin. Return zero — CPU % will read as 0.
	return cpuTimes{}, nil
}

func filesystemUsage(path string) (filesystemStats, error) {
	var stats unix.Statfs_t
	if err := unix.Statfs(path, &stats); err != nil {
		return filesystemStats{}, fmt.Errorf("could not read filesystem stats: %w", err)
	}

	totalBytes := stats.Blocks * uint64(stats.Bsize)
	availableBytes := stats.Bavail * uint64(stats.Bsize)
	usedBytes := totalBytes - availableBytes

	var usedPercent float64
	if totalBytes > 0 {
		usedPercent = float64(usedBytes) / float64(totalBytes) * 100
	}

	return filesystemStats{
		usedMB:      float64(usedBytes) / oneMb,
		totalMB:     float64(totalBytes) / oneMb,
		usedPercent: usedPercent,
	}, nil
}

func diskIOStats() (diskIO, error) {
	// No /proc/diskstats on Darwin. Return zeros.
	return diskIO{}, nil
}

func openTCPConnectionCount() (int, error) {
	// No /proc/net/tcp on Darwin. Return zero.
	return 0, nil
}
