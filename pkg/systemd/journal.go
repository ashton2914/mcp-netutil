package systemd

import (
	"bufio"
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// GetJournalLogs retrieves the logs for a specific unit
// lines: number of lines to retrieve (default 100 if <= 0)
func GetJournalLogs(unit string, lines int) ([]string, error) {
	if unit == "" {
		return nil, fmt.Errorf("unit name cannot be empty")
	}

	if lines <= 0 {
		lines = 100
	}

	// journalctl -u <unit> -n <lines> --no-pager
	cmd := exec.Command("journalctl", "-u", unit, "-n", fmt.Sprintf("%d", lines), "--no-pager")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("failed to run journalctl: %w, stderr: %s", err, stderr.String())
	}

	var logs []string
	scanner := bufio.NewScanner(&stdout)
	for scanner.Scan() {
		logs = append(logs, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading journal output: %w", err)
	}

	// journalctl can return extensive info, just returning lines is compliant with "retrieving the last 1000 entries"
	if len(logs) == 0 {
		return []string{fmt.Sprintf("No logs found for unit '%s'. Please check if the unit name is correct or if it has any logs.", unit)}, nil
	}

	// Handle case where journalctl prints "-- No entries --" or similar
	if len(logs) == 1 && strings.Contains(logs[0], "-- No entries --") {
		return logs, nil
	}

	return logs, nil
}
