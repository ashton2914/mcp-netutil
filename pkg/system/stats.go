package system

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/mem"
)

type SystemStats struct {
	CPU             CPUStats       `json:"cpu"`
	Memory          MemoryStats    `json:"memory"`
	Disk            DiskStats      `json:"disk"`
	TopCPUProcesses []ProcessInfo  `json:"top_cpu_processes,omitempty"`
	TopMemProcesses []ProcessInfo  `json:"top_mem_processes,omitempty"`
	Network         []NetworkStats `json:"network,omitempty"`
}

type CPUStats struct {
	UsagePercent float64 `json:"usage_percent"`
}

type MemoryStats struct {
	Total       uint64  `json:"total"`
	Available   uint64  `json:"available"`
	UsedPercent float64 `json:"used_percent"`
}

type DiskStats struct {
	Path        string  `json:"path"`
	Total       uint64  `json:"total"`
	Free        uint64  `json:"free"`
	UsedPercent float64 `json:"used_percent"`
}

// GetStats collects system statistics including CPU, Memory, Disk usage, top processes and network usage
func GetStats(ctx context.Context) (string, error) {
	// We need to collect stats that require a duration (CPU process, Network) in parallel
	// to minimize total latency.
	var wg sync.WaitGroup
	wg.Add(3)

	var (
		cpuUsage       float64
		cpuErr         error
		topCPU, topMem []ProcessInfo
		procErr        error
		netStats       []NetworkStats
		netErr         error
		scanDuration   = 5 * time.Second
	)

	// 1. Overall System CPU Usage (5s)
	go func() {
		defer wg.Done()
		// Use 5s to match the process scan duration for consistency
		cpuPercent, err := cpu.PercentWithContext(ctx, scanDuration, false)
		if err != nil {
			cpuErr = err
			return
		}
		if len(cpuPercent) > 0 {
			cpuUsage = cpuPercent[0]
		}
	}()

	// 2. Top Processes (5s)
	go func() {
		defer wg.Done()
		topCPU, topMem, procErr = GetProcessStats(scanDuration)
	}()

	// 3. Network Usage (5s)
	go func() {
		defer wg.Done()
		netStats, netErr = GetNetworkUsage(scanDuration)
	}()

	// Wait for all duration-based checks
	wg.Wait()

	// Check for errors in critical parts (logging them might be better than failing all?)
	// For now, if comprehensive info fails, we return error.
	if cpuErr != nil {
		return "", fmt.Errorf("failed to get cpu stats: %w", cpuErr)
	}
	if procErr != nil {
		// Non-critical, just log or ignore?
		// The prompt implies we *should* have it. Let's return error if it fails significantly.
		return "", fmt.Errorf("failed to get process stats: %w", procErr)
	}
	if netErr != nil {
		return "", fmt.Errorf("failed to get network stats: %w", netErr)
	}

	// Instantaneous Checks (Memory, Disk)

	// Virtual Memory
	vMem, err := mem.VirtualMemoryWithContext(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get memory stats: %w", err)
	}

	// Disk Usage
	// Windows uses "C:\", Unix/Linux/macOS uses "/"
	diskPath := "/"
	if runtime.GOOS == "windows" {
		diskPath = "C:\\"
	}

	dUsage, err := disk.UsageWithContext(ctx, diskPath)
	if err != nil {
		return "", fmt.Errorf("failed to get disk usage: %w", err)
	}

	stats := SystemStats{
		CPU: CPUStats{
			UsagePercent: cpuUsage,
		},
		Memory: MemoryStats{
			Total:       vMem.Total,
			Available:   vMem.Available,
			UsedPercent: vMem.UsedPercent,
		},
		Disk: DiskStats{
			Path:        "/",
			Total:       dUsage.Total,
			Free:        dUsage.Free,
			UsedPercent: dUsage.UsedPercent,
		},
		TopCPUProcesses: topCPU,
		TopMemProcesses: topMem,
		Network:         netStats,
	}

	jsonData, err := json.MarshalIndent(stats, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal stats to json: %w", err)
	}

	return string(jsonData), nil
}
