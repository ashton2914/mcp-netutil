package traceroute

import (
	"context"
	"fmt"
	"net"
	"os/exec"
	"runtime"
	"strings"
)

// Run executes the traceroute command for a given target.
func Run(ctx context.Context, target string) (string, error) {
	// Basic validation to prevent command injection
	if err := validateTarget(target); err != nil {
		return "", fmt.Errorf("invalid target: %w", err)
	}

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		// -d: Do not resolve addresses to hostnames (faster)
		// -w: Timeout in milliseconds (reduced to 500ms)
		// -h: Maximum hops (kept at 20)
		cmd = exec.CommandContext(ctx, "tracert", "-d", "-w", "500", "-h", "20", target)
	case "darwin":
		// macOS traceroute
		// -n: Do not resolve IP addresses to hostnames
		// -w: Wait time in seconds (must be int/float depending on version, 1 is safe)
		// -q: Number of queries per hop
		// -m: Max hops
		cmd = exec.CommandContext(ctx, "traceroute", "-n", "-w", "1", "-q", "1", "-m", "20", target)
	default:
		// Linux/Unix
		// -n: Do not resolve IP addresses to hostnames
		// -w: Wait time in seconds
		// -q: Number of queries per hop
		// -m: Max hops
		cmd = exec.CommandContext(ctx, "traceroute", "-n", "-w", "1", "-q", "1", "-m", "20", target)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		// Check if it was a context error
		if ctx.Err() == context.DeadlineExceeded {
			return string(output), fmt.Errorf("traceroute timed out: %w", err)
		}
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
