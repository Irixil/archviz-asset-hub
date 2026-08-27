//go:build linux

package telemetry

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
)

func totalSystemMemoryMB() (float64, error) {
	var info unix.Sysinfo_t
	if err := unix.Sysinfo(&info); err != nil {
		return 0, fmt.Errorf("could not read system memory: %w", err)
	}
	return float64(info.Totalram) * float64(info.Unit) / oneMb, nil
}

func readCPUTimes() (cpuTimes, error) {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return cpuTimes{}, fmt.Errorf("could not read CPU stats: %w", err)
	}

	lines := strings.Split(string(data), "\n")
	if len(lines) == 0 {
		return cpuTimes{}, errors.New("could not read CPU stats: missing cpu line")
	}

	fields := strings.Fields(lines[0])
	if len(fields) < 5 || fields[0] != "cpu" {
		return cpuTimes{}, errors.New("could not read CPU stats: invalid cpu line")
	}

	values := make([]uint64, 0, len(fields)-1)
	for _, field := range fields[1:] {
		value, parseErr := strconv.ParseUint(field, 10, 64)
		if parseErr != nil {
			return cpuTimes{}, fmt.Errorf("could not parse CPU stat %q: %w", field, parseErr)
		}
		values = append(values, value)
	}

	var total uint64
	for _, value := range values {
		total += value
	}

	idle := values[3]
	if len(values) > 4 {
		idle += values[4]
	}

	return cpuTimes{idle: idle, total: total}, nil
}

func filesystemUsage(path string) (filesystemStats, error) {
	var stats unix.Statfs_t
	if err := unix.Statfs(path, &stats); err != nil {
		return filesystemStats{}, fmt.Errorf("could not read filesystem stats: %w", err)
	}

	if stats.Bsize < 0 {
		return filesystemStats{}, fmt.Errorf("unexpected negative block size: %d", stats.Bsize)
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
	data, err := os.ReadFile("/proc/diskstats")
	if err != nil {
		return diskIO{}, fmt.Errorf("could not read disk stats: %w", err)
	}

	var stats diskIO
	for line := range strings.SplitSeq(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 10 || !isWholeDisk(fields[2]) {
			continue
		}

		sectorsRead, parseErr := strconv.ParseUint(fields[5], 10, 64)
		if parseErr != nil {
			return diskIO{}, fmt.Errorf("could not parse disk sectors read for %s: %w", fields[2], parseErr)
		}

		sectorsWritten, parseErr := strconv.ParseUint(fields[9], 10, 64)
		if parseErr != nil {
			return diskIO{}, fmt.Errorf("could not parse disk sectors written for %s: %w", fields[2], parseErr)
		}

		stats.readBytes += sectorsRead * 512
		stats.writeBytes += sectorsWritten * 512
	}

	return stats, nil
}

func openTCPConnectionCount() (int, error) {
	count, err := openTCPConnectionCountForFile("/proc/net/tcp")
	if err != nil {
		return 0, err
	}

	ipv6Count, err := openTCPConnectionCountForFile("/proc/net/tcp6")
	if err != nil {
		return 0, err
	}

	return count + ipv6Count, nil
}

func openTCPConnectionCountForFile(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, fmt.Errorf("could not read TCP connections from %s: %w", path, err)
	}

	count := 0
	for index, line := range strings.Split(string(data), "\n") {
		if index == 0 || strings.TrimSpace(line) == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}

		if fields[3] != "0A" {
			count++
		}
	}

	return count, nil
}

func isWholeDisk(name string) bool {
	if strings.HasPrefix(name, "loop") || strings.HasPrefix(name, "ram") || strings.HasPrefix(name, "fd") ||
		strings.HasPrefix(name, "sr") {
		return false
	}
	if strings.HasPrefix(name, "dm-") || strings.HasPrefix(name, "md") {
		return true
	}
	if strings.HasPrefix(name, "nvme") {
		return !strings.Contains(name, "p")
	}
	if strings.HasPrefix(name, "mmcblk") {
		return !strings.Contains(name, "p")
	}

	last := name[len(name)-1]
	return last < '0' || last > '9'
}
