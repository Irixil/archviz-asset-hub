//go:build !linux && !darwin

package telemetry

func totalSystemMemoryMB() (float64, error) { return 0, nil }
func readCPUTimes() (cpuTimes, error)        { return cpuTimes{}, nil }
func filesystemUsage(_ string) (filesystemStats, error) {
	return filesystemStats{}, nil
}
func diskIOStats() (diskIO, error)       { return diskIO{}, nil }
func openTCPConnectionCount() (int, error) { return 0, nil }
