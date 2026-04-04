package executor

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"time"
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

// MergeFiles runs mkvmerge and reports coarse progress from the output file size.
// onProgress may be nil.
func MergeFiles(parts []string, output string, totalSize int64, onProgress func(int)) error {
	if len(parts) < 2 {
		return fmt.Errorf("need at least 2 parts to merge")
	}
	cmd := BuildMergeCommand(parts, output)
	var outputLog bytes.Buffer
	cmd.Stdout = &outputLog
	cmd.Stderr = &outputLog

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("mkvmerge start: %w", err)
	}

	done := make(chan struct{})
	if onProgress != nil && totalSize > 0 {
		go pollMergeProgress(output, totalSize, onProgress, done)
	}

	err := cmd.Wait()
	close(done)
	if err != nil {
		// mkvmerge exit codes: 0 = success, 1 = warnings (ok), 2 = error
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			// warnings only, merge succeeded
		} else {
			return fmt.Errorf("mkvmerge failed: %w\n%s", err, outputLog.String())
		}
	}
	if onProgress != nil {
		onProgress(100)
	}
	return nil
}

func pollMergeProgress(output string, totalSize int64, onProgress func(int), done <-chan struct{}) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	lastPct := -1
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			fi, err := os.Stat(output)
			if err != nil {
				if !os.IsNotExist(err) {
					continue
				}
				if lastPct != 0 {
					lastPct = 0
					onProgress(0)
				}
				continue
			}
			pct := mergeProgressFromSize(fi.Size(), totalSize)
			if pct != lastPct {
				lastPct = pct
				onProgress(pct)
			}
		}
	}
}

func mergeProgressFromSize(currentSize, totalSize int64) int {
	if totalSize <= 0 || currentSize <= 0 {
		return 0
	}
	pct := int(currentSize * 100 / totalSize)
	if pct > 99 {
		return 99
	}
	return pct
}
