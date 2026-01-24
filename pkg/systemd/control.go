package systemd

import (
	"fmt"
	"os/exec"
)

// ControlService manages systemd services using systemctl
// Supported actions: start, stop, restart, reload, enable, disable, status
func ControlService(unit string, action string) (string, error) {
	if unit == "" {
		return "", fmt.Errorf("unit name cannot be empty")
	}

	allowedActions := map[string]bool{
		"start":   true,
		"stop":    true,
		"restart": true,
		"reload":  true,
		"enable":  true,
		"disable": true,
		"status":  true,
	}

	if !allowedActions[action] {
		return "", fmt.Errorf("invalid action '%s'. Allowed actions: start, stop, restart, reload, enable, disable, status", action)
	}

	// systemctl <action> <unit>
	cmd := exec.Command("systemctl", action, unit)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// specific handling: 'status' returns non-zero exit code if service is stopped/failed, but we still want the output
		if action == "status" {
			return string(output), nil
		}
		return string(output), fmt.Errorf("failed to execute systemctl %s %s: %w, output: %s", action, unit, err, string(output))
	}

	return string(output), nil
}
