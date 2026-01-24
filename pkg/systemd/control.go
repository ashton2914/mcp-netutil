package systemd

import (
	"fmt"
	"os/exec"
)

// ControlService manages systemd services using systemctl
// Supported actions: start, stop, restart, reload, enable, disable
func ControlService(unit string, action string) error {
	if unit == "" {
		return fmt.Errorf("unit name cannot be empty")
	}

	allowedActions := map[string]bool{
		"start":   true,
		"stop":    true,
		"restart": true,
		"reload":  true,
		"enable":  true,
		"disable": true,
	}

	if !allowedActions[action] {
		return fmt.Errorf("invalid action '%s'. Allowed actions: start, stop, restart, reload, enable, disable", action)
	}

	// systemctl <action> <unit>
	cmd := exec.Command("systemctl", action, unit)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to execute systemctl %s %s: %w, output: %s", action, unit, err, string(output))
	}

	return nil
}
