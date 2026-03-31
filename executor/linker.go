package executor

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
)

// LinkResult holds the outcome of a link operation.
type LinkResult struct {
	LinkPath string
	LinkType string // actual link type used ("hardlink" or "symlink")
}

// LinkFile creates a link from srcPath into outDir.
// When multiPart is true, the filename includes a -cd{part} suffix (e.g. SIVR-476-cd1.mp4).
// Returns the actual link type used (may differ from requested if hardlink falls back to symlink).
func LinkFile(srcPath, outDir, number, linkType string, multiPart bool, part int) (*LinkResult, error) {
	ext := filepath.Ext(srcPath)
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir %s: %w", outDir, err)
	}

	name := number
	if multiPart {
		name = fmt.Sprintf("%s-cd%d", number, part)
	}
	linkPath := filepath.Join(outDir, name+ext)
	_ = os.Remove(linkPath)

	actualType := linkType
	switch linkType {
	case "hardlink":
		if err := os.Link(srcPath, linkPath); err != nil {
			log.Printf("hardlink failed, falling back to symlink: %v", err)
			actualType = "symlink"
			if err := os.Symlink(srcPath, linkPath); err != nil {
				return nil, fmt.Errorf("symlink fallback: %w", err)
			}
		}
	case "symlink":
		if err := os.Symlink(srcPath, linkPath); err != nil {
			return nil, fmt.Errorf("symlink: %w", err)
		}
	default:
		return nil, fmt.Errorf("unknown link type: %s", linkType)
	}

	return &LinkResult{LinkPath: linkPath, LinkType: actualType}, nil
}
