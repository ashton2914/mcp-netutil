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

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		// Windows: -n count, -w timeout (ms)
		// We'll set a reasonable timeout per reply, e.g., 1000ms
		args := []string{"-n", count, "-w", "1000", target}
		cmd = exec.CommandContext(ctx, "ping", args...)
	} else {
		// Linux/macOS: -c count, -i interval (0.2s), -q quiet
		args := []string{"-c", count, "-i", "0.2", "-q", target}
		cmd = exec.CommandContext(ctx, "ping", args...)
	}

	outputBytes, err := cmd.CombinedOutput()
	output := string(outputBytes)
	if err != nil {
		// ping returns non-zero if there is any packet loss or timeout.
		// We still try to parse statistics if some packets were received.
		if len(output) == 0 {
			return nil, fmt.Errorf("ping failed: %w", err)
		}
	}

	return parsePingOutput(output, mode)
}

func parsePingOutput(output string, mode string) (LatencyResult, error) {
	result := LatencyResult{}

	// --- Packet Loss Parsing ---
	// Linux/macOS: "X% packet loss"
	// Windows: "Lost = X (Y% loss)"
	lossRegexUnix := regexp.MustCompile(`(\d+(?:\.\d+)?)% packet loss`)
	lossRegexWin := regexp.MustCompile(`\((\d+)% loss\)`)

	if match := lossRegexUnix.FindStringSubmatch(output); len(match) > 1 {
		result.PacketLoss = match[1] + "%"
	} else if match := lossRegexWin.FindStringSubmatch(output); len(match) > 1 {
		result.PacketLoss = match[1] + "%"
	}

	// --- Latency Parsing ---
	// Linux: "rtt min/avg/max/mdev = 14.123/14.567/15.890/0.987 ms"
	// macOS: "round-trip min/avg/max/stddev = 14.123/14.567/15.890/0.987 ms"
	// Windows: "Minimum = 14ms, Maximum = 16ms, Average = 15ms"

	// Unified Unix Regex (handles 'rtt' and 'round-trip', 'mdev' and 'stddev')
	rttRegexUnix := regexp.MustCompile(`(?:rtt|round-trip) min/avg/max/(?:mdev|stddev) = ([0-9.]+)/([0-9.]+)/([0-9.]+)/([0-9.]+) ms`)
	// Windows Regex
	rttRegexWin := regexp.MustCompile(`Minimum = (\d+)ms, Maximum = (\d+)ms, Average = (\d+)ms`)

	if match := rttRegexUnix.FindStringSubmatch(output); len(match) > 4 {
		// match[1]=min, match[2]=avg, match[3]=max, match[4]=dev
		result.AvgLatency = match[2] + " ms"
		if mode != "quick" {
			result.Jitter = match[4] + " ms"
		}
	} else if match := rttRegexWin.FindStringSubmatch(output); len(match) > 3 {
		// match[1]=min, match[2]=max, match[3]=avg
		result.AvgLatency = match[3] + " ms"
		if mode != "quick" {
			// Windows ping doesn't provide jitter (stddev) directly.
			// We could convert min/max to integers and estimate, but better to leave empty or "N/A"
			result.Jitter = "N/A"
		}
	} else {
		// Failure to parse latency
		// If 100% packet loss, AvgLatency might not be present.
		if result.PacketLoss == "100%" {
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
