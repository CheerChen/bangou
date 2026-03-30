package executor

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
)

func BuildMergeCommand(parts []string, output string) *exec.Cmd {
	args := []string{"-o", output}
	for i, p := range parts {
		if i > 0 {
			args = append(args, "+")
		}
		args = append(args, p)
	}
	return exec.Command("mkvmerge", args...)
}

// MergeFiles runs mkvmerge and calls onProgress(0-100) as progress updates arrive.
// onProgress may be nil.
func MergeFiles(parts []string, output string, onProgress func(int)) error {
	if len(parts) < 2 {
		return fmt.Errorf("need at least 2 parts to merge")
	}
	cmd := BuildMergeCommand(parts, output)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}
	cmd.Stderr = cmd.Stdout // merge stderr into stdout

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("mkvmerge start: %w", err)
	}

	// Parse "Progress: 45%" lines in real time
	scanner := bufio.NewScanner(stdout)
	var lastOutput strings.Builder
	for scanner.Scan() {
		line := scanner.Text()
		lastOutput.WriteString(line)
		lastOutput.WriteByte('\n')
		if pct, ok := parseProgress(line); ok && onProgress != nil {
			onProgress(pct)
		}
	}

	// Drain remaining output
	remaining, _ := io.ReadAll(stdout)
	lastOutput.Write(remaining)

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("mkvmerge failed: %w\n%s", err, lastOutput.String())
	}
	return nil
}

// parseProgress extracts percentage from "Progress: 45%" lines.
func parseProgress(line string) (int, bool) {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "Progress:") {
		return 0, false
	}
	s := strings.TrimPrefix(line, "Progress:")
	s = strings.TrimSpace(s)
	s = strings.TrimSuffix(s, "%")
	s = strings.TrimSpace(s)
	pct, err := strconv.Atoi(s)
	if err != nil {
		return 0, false
	}
	return pct, true
}
