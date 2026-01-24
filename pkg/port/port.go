package port

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

type PortStatus struct {
	Port     int    `json:"port"`
	Protocol string `json:"protocol"`
	State    string `json:"state"`
	Process  string `json:"process"` // e.g., "nginx (pid=1234)"
}

// GetPortStatus returns the status of a specific port.
// If port is 0, it returns all listening ports.
func GetPortStatus(ctx context.Context, port int) ([]PortStatus, error) {
	// Use ss -tulnpr to list tcp/udp, listening, numeric, processes
	// -t: tcp, -u: udp, -l: listening, -n: numeric, -p: processes, -H: no header
	// Note: -H might not be available on all ss versions, so we'll parse carefully.
	args := []string{"-tulnpH"}

	cmd := exec.CommandContext(ctx, "ss", args...)
	outputBytes, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("ss command failed: %w", err)
	}

	output := string(outputBytes)
	lines := strings.Split(output, "\n")
	var results []PortStatus

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		// Expected format (roughly):
		// Netid State  Recv-Q Send-Q Local Address:Port  Peer Address:PortProcess
		// udp   UNCONN 0      0      0.0.0.0:1234       0.0.0.0:*      users:(("process_name",pid=123,fd=4))

		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue // skip invalid lines
		}

		protocol := fields[0]
		state := fields[1]
		localAddr := fields[4]

		// Parse Local Address to get Port
		// Format could be 127.0.0.1:80 or [::]:80
		lastColon := strings.LastIndex(localAddr, ":")
		if lastColon == -1 {
			continue
		}

		portStr := localAddr[lastColon+1:]
		p, err := strconv.Atoi(portStr)
		if err != nil {
			continue
		}

		// Filter if specific port requested
		if port != 0 && p != port {
			continue
		}

		// Parse Process Info
		processInfo := ""
		if len(fields) > 6 {
			// users:(("nginx",pid=1234,fd=6))
			rawProc := strings.Join(fields[6:], " ")
			// Extract meaningful info
			// Regex to extract name and pid
			re := regexp.MustCompile(`users:\(\("([^"]+)",pid=(\d+),`)
			matches := re.FindStringSubmatch(rawProc)
			if len(matches) == 3 {
				processInfo = fmt.Sprintf("%s (pid=%s)", matches[1], matches[2])
			} else {
				processInfo = rawProc // Fallback
			}
		}

		results = append(results, PortStatus{
			Port:     p,
			Protocol: protocol,
			State:    state,
			Process:  processInfo,
		})
	}

	return results, nil
}
