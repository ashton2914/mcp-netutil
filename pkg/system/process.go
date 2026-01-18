package system

import (
	"fmt"
	"sort"
	"time"

	"github.com/shirou/gopsutil/v4/net"
	"github.com/shirou/gopsutil/v4/process"
)

type ProcessInfo struct {
	PID  int32  `json:"pid"`
	Name string `json:"name"`
	Val  string `json:"value"` // Formatted value (e.g. "12.5%" or "1024 MB")
}

// KillProcess terminates a process by its PID
func KillProcess(pid int32) error {
	p, err := process.NewProcess(pid)
	if err != nil {
		return fmt.Errorf("process not found: %w", err)
	}
	return p.Kill()
}

// KillProcessByName terminates a process by its name
// It kills ALL processes matching the name
func KillProcessByName(name string) error {
	procs, err := process.Processes()
	if err != nil {
		return fmt.Errorf("failed to list processes: %w", err)
	}

	killed := 0
	for _, p := range procs {
		pName, err := p.Name()
		if err != nil {
			continue // Skip processes we can't access
		}
		if pName == name {
			if err := p.Kill(); err != nil {
				return fmt.Errorf("failed to kill process %d (%s): %w", p.Pid, pName, err)
			}
			killed++
		}
	}

	if killed == 0 {
		return fmt.Errorf("no process found with name %s", name)
	}

	return nil
}

// GetProcessStats returns the top 10 CPU and Memory consuming processes
// It monitors CPU usage over the specified duration
func GetProcessStats(duration time.Duration) (topCPU []ProcessInfo, topMem []ProcessInfo, err error) {
	procs, err := process.Processes()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to list processes: %w", err)
	}

	// Wrapper to hold process and its stats
	type procStats struct {
		pid  int32
		name string
		p    *process.Process
		cpu  float64
		mem  float32
	}

	// Create wrappers and initialize
	wrappers := make([]*procStats, 0, len(procs))
	for _, p := range procs {
		// Initialize CPU counter
		p.Percent(0)
		wrappers = append(wrappers, &procStats{p: p, pid: p.Pid})
	}

	time.Sleep(duration)

	for _, w := range wrappers {
		// Check CPU
		if c, err := w.p.Percent(0); err == nil {
			w.cpu = c
		}

		// Check Memory (instant is fine)
		if m, err := w.p.MemoryPercent(); err == nil {
			w.mem = m
		}

		// Try to get name
		if n, err := w.p.Name(); err == nil {
			w.name = n
		} else {
			w.name = "unknown"
		}
	}

	// Sort and extract top 10 CPU
	sort.Slice(wrappers, func(i, j int) bool {
		return wrappers[i].cpu > wrappers[j].cpu
	})

	count := 10
	if len(wrappers) < 10 {
		count = len(wrappers)
	}

	topCPU = make([]ProcessInfo, 0, count)
	for i := 0; i < count; i++ {
		w := wrappers[i]
		// Filter out 0 usage if undesired, but typically top processes might include low usage if system is idle
		topCPU = append(topCPU, ProcessInfo{
			PID:  w.pid,
			Name: w.name,
			Val:  fmt.Sprintf("%.2f%%", w.cpu),
		})
	}

	// Sort and extract top 10 Memory
	sort.Slice(wrappers, func(i, j int) bool {
		return wrappers[i].mem > wrappers[j].mem
	})

	topMem = make([]ProcessInfo, 0, count)
	for i := 0; i < count; i++ {
		w := wrappers[i]
		topMem = append(topMem, ProcessInfo{
			PID:  w.pid,
			Name: w.name,
			Val:  fmt.Sprintf("%.2f%%", w.mem),
		})
	}

	return topCPU, topMem, nil
}

type NetworkStats struct {
	Interface string `json:"interface"`
	Rx        string `json:"rx"` // e.g. "10.5 KB/s"
	Tx        string `json:"tx"` // e.g. "5.2 KB/s"
}

// GetNetworkUsage returns network usage per interface over the duration
func GetNetworkUsage(duration time.Duration) ([]NetworkStats, error) {
	startStats, err := net.IOCounters(true)
	if err != nil {
		return nil, err
	}

	time.Sleep(duration)

	endStats, err := net.IOCounters(true)
	if err != nil {
		return nil, err
	}

	var results []NetworkStats
	seconds := duration.Seconds()

	for _, end := range endStats {
		for _, start := range startStats {
			if end.Name == start.Name {
				rxBytes := end.BytesRecv - start.BytesRecv
				txBytes := end.BytesSent - start.BytesSent

				rxRate := float64(rxBytes) / seconds
				txRate := float64(txBytes) / seconds

				results = append(results, NetworkStats{
					Interface: end.Name,
					Rx:        humanizeBytes(rxRate) + "/s",
					Tx:        humanizeBytes(txRate) + "/s",
				})
				break
			}
		}
	}

	// Sort by Name
	sort.Slice(results, func(i, j int) bool {
		return results[i].Interface < results[j].Interface
	})

	return results, nil
}

func humanizeBytes(s float64) string {
	sizes := []string{"B", "KB", "MB", "GB", "TB"}
	i := 0
	for s >= 1024 && i < len(sizes)-1 {
		s /= 1024
		i++
	}
	return fmt.Sprintf("%.1f %s", s, sizes[i])
}
