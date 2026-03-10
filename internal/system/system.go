package system

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Info struct {
	CPUPct     float64 `json:"cpu_pct"`
	RAMUsedMB  int     `json:"ram_used_mb"`
	RAMTotalMB int     `json:"ram_total_mb"`
	DiskUsedGB float64 `json:"disk_used_gb"`
	DiskTotalGB float64 `json:"disk_total_gb"`
	LoadAvg1   float64 `json:"load_avg_1"`
	LoadAvg5   float64 `json:"load_avg_5"`
	LoadAvg15  float64 `json:"load_avg_15"`
	UptimeSec  int     `json:"uptime_sec"`
	Hostname   string  `json:"hostname"`
}

// Read collects system metrics from /proc
func Read() (Info, error) {
	var info Info

	// Hostname
	if data, err := os.ReadFile("/proc/sys/kernel/hostname"); err == nil {
		info.Hostname = strings.TrimSpace(string(data))
	}

	// Uptime
	if data, err := os.ReadFile("/proc/uptime"); err == nil {
		parts := strings.Fields(string(data))
		if len(parts) > 0 {
			if v, err := strconv.ParseFloat(parts[0], 64); err == nil {
				info.UptimeSec = int(v)
			}
		}
	}

	// Load average
	if data, err := os.ReadFile("/proc/loadavg"); err == nil {
		parts := strings.Fields(string(data))
		if len(parts) >= 3 {
			info.LoadAvg1, _ = strconv.ParseFloat(parts[0], 64)
			info.LoadAvg5, _ = strconv.ParseFloat(parts[1], 64)
			info.LoadAvg15, _ = strconv.ParseFloat(parts[2], 64)
		}
	}

	// Memory from /proc/meminfo
	if data, err := os.ReadFile("/proc/meminfo"); err == nil {
		mem := parseMeminfo(string(data))
		info.RAMTotalMB = mem["MemTotal"] / 1024
		available := mem["MemAvailable"]
		info.RAMUsedMB = info.RAMTotalMB - available/1024
	}

	// CPU usage (two samples 500ms apart)
	cpu1, err1 := readCPUStat()
	if err1 == nil {
		time.Sleep(500 * time.Millisecond)
		cpu2, err2 := readCPUStat()
		if err2 == nil {
			totalDelta := cpu2.total - cpu1.total
			idleDelta := cpu2.idle - cpu1.idle
			if totalDelta > 0 {
				info.CPUPct = float64(totalDelta-idleDelta) / float64(totalDelta) * 100
			}
		}
	}

	// Disk usage from /proc/mounts + statfs would need syscall
	// Use simple approach: read from statvfs via /proc/diskstats is complex
	// Instead parse df-like info from /proc/self/mountinfo
	info.DiskUsedGB, info.DiskTotalGB = readDiskUsage("/")

	return info, nil
}

type cpuStat struct {
	total uint64
	idle  uint64
}

func readCPUStat() (cpuStat, error) {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return cpuStat{}, err
	}
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "cpu ") {
			fields := strings.Fields(line)
			if len(fields) < 5 {
				return cpuStat{}, fmt.Errorf("unexpected /proc/stat format")
			}
			var total, idle uint64
			for i := 1; i < len(fields); i++ {
				v, _ := strconv.ParseUint(fields[i], 10, 64)
				total += v
				if i == 4 { // idle is 4th field (1-indexed after "cpu")
					idle = v
				}
			}
			return cpuStat{total: total, idle: idle}, nil
		}
	}
	return cpuStat{}, fmt.Errorf("cpu line not found")
}

func parseMeminfo(data string) map[string]int {
	result := make(map[string]int)
	for _, line := range strings.Split(data, "\n") {
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			key := strings.TrimSuffix(parts[0], ":")
			val, _ := strconv.Atoi(parts[1])
			result[key] = val // values in kB
		}
	}
	return result
}

func readDiskUsage(path string) (usedGB, totalGB float64) {
	// Use syscall.Statfs on Linux
	var stat syscallStatfs
	if err := statfs(path, &stat); err != nil {
		return 0, 0
	}
	totalGB = float64(stat.Blocks*uint64(stat.Bsize)) / (1024 * 1024 * 1024)
	freeGB := float64(stat.Bavail*uint64(stat.Bsize)) / (1024 * 1024 * 1024)
	usedGB = totalGB - freeGB
	return usedGB, totalGB
}
