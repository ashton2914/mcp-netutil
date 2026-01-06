package system

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/mem"
)

type SystemStats struct {
	CPU    CPUStats    `json:"cpu"`
	Memory MemoryStats `json:"memory"`
	Disk   DiskStats   `json:"disk"`
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

// GetStats collects system statistics including CPU, Memory, and Disk usage
func GetStats(ctx context.Context) (string, error) {
	// CPU Usage (Total percentage)
	// interval of 0 means use the time since the last call or system boot, but gopsutil recommends at least a small interval for accurate instant reading if not continuously polling.
	// However, for a one-off tool call, we might need a small duration to measure 'current' load, or 0 for "since last call".
	// If 0 is used for the first time, it might return 0 or error on some systems, or the average since boot.
	// A small sleep like 100ms or 1s is often needed for accurate "current" CPU usage.
	// We will use 500ms which offers a balance between responsiveness and accuracy.
	cpuPercent, err := cpu.PercentWithContext(ctx, 500*time.Millisecond, false)
	if err != nil {
		return "", fmt.Errorf("failed to get cpu stats: %w", err)
	}

	var cpuUsage float64
	if len(cpuPercent) > 0 {
		cpuUsage = cpuPercent[0]
	}

	// Virtual Memory
	vMem, err := mem.VirtualMemoryWithContext(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get memory stats: %w", err)
	}

	// Disk Usage for root "/"
	dUsage, err := disk.UsageWithContext(ctx, "/")
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
	}

	jsonData, err := json.MarshalIndent(stats, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal stats to json: %w", err)
	}

	return string(jsonData), nil
}
