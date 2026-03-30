package executor

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
)

// LinkFile creates a link from srcPath into outDir.
// When multiPart is true, the filename includes a -cd{part} suffix (e.g. SIVR-476-cd1.mp4).
func LinkFile(srcPath, outDir, number, linkType string, multiPart bool, part int) (string, error) {
	ext := filepath.Ext(srcPath)
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return "", fmt.Errorf("mkdir %s: %w", outDir, err)
	}

	name := number
	if multiPart {
		name = fmt.Sprintf("%s-cd%d", number, part)
	}
	linkPath := filepath.Join(outDir, name+ext)
	_ = os.Remove(linkPath)

	switch linkType {
	case "hardlink":
		if err := os.Link(srcPath, linkPath); err != nil {
			// Fallback to symlink (e.g. cross-device in Docker bind mounts)
			log.Printf("hardlink failed, falling back to symlink: %v", err)
			if err := os.Symlink(srcPath, linkPath); err != nil {
				return "", fmt.Errorf("symlink fallback: %w", err)
			}
		}
	case "symlink":
		if err := os.Symlink(srcPath, linkPath); err != nil {
			return "", fmt.Errorf("symlink: %w", err)
		}
	default:
		return "", fmt.Errorf("unknown link type: %s", linkType)
	}

	return linkPath, nil
}
