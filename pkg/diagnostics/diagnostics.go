package diagnostics

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// DiagnosticsResult holds the result of all diagnostic checks
type DiagnosticsResult struct {
	JournalctlErrors []string `json:"journalctl_errors"`
	SyslogErrors     []string `json:"syslog_errors"`
	Dmesg            []string `json:"dmesg"`
	LoginHistory     []string `json:"login_history"`
	FailedLogins     []string `json:"failed_logins"`
}

// RunDiagnostics gathers system diagnostic information
func RunDiagnostics() (*DiagnosticsResult, error) {
	res := &DiagnosticsResult{}
	var err error

	// 1. Journalctl Errors (last 100 entries, priority err(3))
	res.JournalctlErrors, err = getCommandOutput("journalctl", "-p", "3", "-n", "100", "--no-pager")
	if err != nil {
		res.JournalctlErrors = []string{fmt.Sprintf("Error running journalctl: %v", err)}
	} else if len(res.JournalctlErrors) == 0 {
		res.JournalctlErrors = []string{"No journalctl error logs found."}
	}

	// 2. Syslog Errors (read file, grep "error" (insensitive), last 100)
	// Typical path: /var/log/syslog. On some systems (RHEL/CentOS) it might be /var/log/messages.
	// We will try /var/log/syslog first.
	res.SyslogErrors = getSyslogErrors("/var/log/syslog", 100)

	// 3. Dmesg (last 50 entries)
	// dmesg might require root or specific capabilities.
	// Using "dmesg | tail -n 50" logic
	dmesgOut, err := getCommandOutput("dmesg")
	if err != nil {
		res.Dmesg = []string{fmt.Sprintf("Error running dmesg: %v", err)}
	} else {
		if len(dmesgOut) > 50 {
			res.Dmesg = dmesgOut[len(dmesgOut)-50:]
		} else {
			res.Dmesg = dmesgOut
		}
	}

	// 4. Last (Login History), last 10
	res.LoginHistory, err = getCommandOutput("last", "-n", "10")
	if err != nil {
		res.LoginHistory = []string{fmt.Sprintf("Error running last: %v", err)}
	}

	// 5. Lastb (Failed Login Attempts), last 10
	// This usually requires root reading /var/log/btmp
	res.FailedLogins, err = getCommandOutput("lastb", "-n", "10")
	if err != nil {
		res.FailedLogins = []string{fmt.Sprintf("Error running lastb: %v", err)}
	}

	return res, nil
}

// getCommandOutput executes a command and returns lines as a slice
func getCommandOutput(name string, args ...string) ([]string, error) {
	cmd := exec.Command(name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// return partial output if available? Or just error.
		// For diagnostics, maybe return error but also check stderr?
		return nil, fmt.Errorf("%w: %s", err, stderr.String())
	}

	var lines []string
	scanner := bufio.NewScanner(&stdout)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, nil
}

// getSyslogErrors reads the syslog file and filters for "error"
func getSyslogErrors(path string, count int) []string {
	file, err := os.Open(path)
	if err != nil {
		return []string{fmt.Sprintf("Could not open %s: %v", path, err)}
	}
	defer file.Close()

	var matchedLines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(strings.ToLower(line), "error") {
			matchedLines = append(matchedLines, line)
		}
	}

	if len(matchedLines) == 0 {
		return []string{fmt.Sprintf("No error lines found in %s", path)}
	}

	if len(matchedLines) > count {
		return matchedLines[len(matchedLines)-count:]
	}
	return matchedLines
}
