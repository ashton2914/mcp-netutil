package traceroute

import (
	"fmt"
	"net"
	"os/exec"
	"runtime"
	"strings"
)

// Run executes the traceroute command for a given target.
func Run(target string) (string, error) {
	// Basic validation to prevent command injection
	if err := validateTarget(target); err != nil {
		return "", fmt.Errorf("invalid target: %w", err)
	}

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		// -w: Timeout in milliseconds
		// -h: Maximum hops
		cmd = exec.Command("tracert", "-w", "1000", "-h", "20", target)
	default:
		// -w: Wait time in seconds
		// -q: Number of queries per hop
		// -m: Max hops
		cmd = exec.Command("traceroute", "-w", "1", "-q", "1", "-m", "20", target)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("traceroute failed: %w", err)
	}

	return string(output), nil
}

func validateTarget(target string) error {
	// Remove protocol if present (e.g., http://, https://) for cleaner host extraction
	// traceroute works on hostnames or IPs. A simple way is to check if it parses as an IP or a valid domain char set.
	// For simplicity, we just check for whitespace or shell characters to prevent injection.
	// A more robust check might try net.LookupHost, but that might fail if DNS is down.

	if target == "" {
		return fmt.Errorf("target cannot be empty")
	}

	if strings.ContainsAny(target, ";&|`$<>") {
		return fmt.Errorf("invalid characters in target")
	}

	// Optional: Check if it's a valid IP
	if net.ParseIP(target) != nil {
		return nil
	}

	// Assume hostname if not IP, just rudimentary check for length
	if len(target) > 253 {
		return fmt.Errorf("target too long")
	}

	return nil
}
