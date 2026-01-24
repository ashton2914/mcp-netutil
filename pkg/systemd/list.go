package systemd

import (
	"fmt"
	"os/exec"
)

// ListUnits returns a list of loaded systemd units (services)
// Wraps: systemctl list-units --type=service --all --no-pager
func ListUnits() (string, error) {
	cmd := exec.Command("systemctl", "list-units", "--type=service", "--all", "--no-pager")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to execute systemctl list-units: %w, output: %s", err, string(output))
	}
	return string(output), nil
}

// ListUnitFiles returns a list of installed systemd unit files (services)
// Wraps: systemctl list-unit-files --type=service --no-pager
func ListUnitFiles() (string, error) {
	cmd := exec.Command("systemctl", "list-unit-files", "--type=service", "--no-pager")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to execute systemctl list-unit-files: %w, output: %s", err, string(output))
	}
	return string(output), nil
}
