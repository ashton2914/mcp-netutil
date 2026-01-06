package latency

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
)

type LatencyResult struct {
	AvgLatency string `json:"avg_latency"` // string to preserve unit or format
	Jitter     string `json:"jitter,omitempty"`
	PacketLoss string `json:"packet_loss,omitempty"`
}

// Run executes the ping command based on the specified mode.
func Run(ctx context.Context, target string, mode string) (interface{}, error) {
	if mode == "" {
		return "Please specify the test mode: 'quick' (10 packets) or 'standard' (100 packets).", nil
	}

	var count string
	switch strings.ToLower(mode) {
	case "quick":
		count = "10"
	case "standard":
		count = "100"
	default:
		return "Invalid mode. Please specify: 'quick' or 'standard'.", nil
	}

	// -c count
	// -i 0.2 (interval 200ms to speed up)
	// -q (quiet output, only summary)
	args := []string{"-c", count, "-i", "0.2", "-q", target}

	// Adjust for Windows if necessary, but user is on Linux.
	if runtime.GOOS == "windows" {
		// Windows ping is different, but strict requirement "User's OS version is linux"
		return nil, fmt.Errorf("windows not supported for this tool yet")
	}

	cmd := exec.CommandContext(ctx, "ping", args...)
	outputBytes, err := cmd.CombinedOutput()
	output := string(outputBytes)
	if err != nil {
		// ping returns non-zero if there is any packet loss or timeout, but we still want the stats if available.
		// However, combined output might contain "unknown host" or similar.
		// We try to parse anyway if we got output.
		if len(output) == 0 {
			return nil, fmt.Errorf("ping failed: %w", err)
		}
	}

	return parsePingOutput(output, mode)
}

func parsePingOutput(output string, mode string) (LatencyResult, error) {
	result := LatencyResult{}

	// Parse Packet Loss
	// Example: "10 packets transmitted, 10 received, 0% packet loss, time 2002ms"
	lossRegex := regexp.MustCompile(`(\d+(?:\.\d+)?)% packet loss`)
	lossMatch := lossRegex.FindStringSubmatch(output)
	if len(lossMatch) > 1 {
		result.PacketLoss = lossMatch[1] + "%"
	}

	// Parse RTT (min/avg/max/mdev)
	// Example: "rtt min/avg/max/mdev = 14.123/14.567/15.890/0.987 ms"
	rttRegex := regexp.MustCompile(`rtt min/avg/max/mdev = ([0-9.]+)/([0-9.]+)/([0-9.]+)/([0-9.]+) ms`)
	rttMatch := rttRegex.FindStringSubmatch(output)

	if len(rttMatch) > 4 {
		// rttMatch[1] = min
		// rttMatch[2] = avg
		// rttMatch[3] = max
		// rttMatch[4] = mdev (jitter)

		result.AvgLatency = rttMatch[2] + " ms"

		if mode != "quick" {
			result.Jitter = rttMatch[4] + " ms"
		}
	} else {
		// If we couldn't parse RTT but we have 100% packet loss, return that
		if result.PacketLoss == "100%" {
			// If mode is standard or full, we return the packet loss.
			// Even in quick mode, if it's 100% loss, we probably should report it or at least not fail with "parse error".
			// But the struct for quick mode only has AvgLatency usually?
			// The current struct definition has PacketLoss as omitempty.
			// If we return just PacketLoss, that's valid useful info.

			// For consistency, let's explicitly set fields to indicate failure if needed,
			// but simply returning the result (which has PacketLoss set) is enough.
			return result, nil
		}
		return result, fmt.Errorf("could not parse ping statistics")
	}

	// Filter based on mode
	finalResult := LatencyResult{
		AvgLatency: result.AvgLatency,
	}

	if mode == "standard" {
		finalResult.Jitter = result.Jitter
		finalResult.PacketLoss = result.PacketLoss
	}

	return finalResult, nil
}
