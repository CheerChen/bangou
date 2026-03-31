package checker

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/CheerChen/bangou/committed"
)

func CheckLink(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// detectActualLinkType checks whether the file at path is a symlink or hardlink.
func detectActualLinkType(path string) string {
	fi, err := os.Lstat(path)
	if err != nil {
		return ""
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		return "symlink"
	}
	return "hardlink"
}

func Run(ctx context.Context, s committed.Store, interval time.Duration) {
	// Fix link types and src_path on startup
	fixOutputRecords(ctx, s)

	t := time.NewTicker(interval)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			check(ctx, s)
		}
	}
}

func check(ctx context.Context, s committed.Store) {
	outputs, err := s.ListAllOutputs(ctx)
	if err != nil {
		log.Printf("checker list outputs: %v", err)
		return
	}
	for _, o := range outputs {
		alive := CheckLink(o.LinkPath)
		if alive == o.Alive {
			continue
		}
		if err := s.SetOutputAlive(ctx, o.ID, alive); err != nil {
			log.Printf("checker update output(%d): %v", o.ID, err)
			continue
		}
		if !alive {
			log.Printf("orphaned: %s -> %s", o.Number, o.LinkPath)
		}
	}
}

// fixOutputRecords scans all outputs and corrects link_type and src_path
// to match what's actually on disk.
func fixOutputRecords(ctx context.Context, s committed.Store) {
	outputs, err := s.ListAllOutputs(ctx)
	if err != nil {
		log.Printf("checker fix outputs: %v", err)
		return
	}
	fixed := 0
	for _, o := range outputs {
		actual := detectActualLinkType(o.LinkPath)
		if actual == "" {
			continue
		}

		// Fix link type
		if actual != o.LinkType {
			if err := s.SetOutputLinkType(ctx, o.ID, actual); err != nil {
				log.Printf("checker fix link type(%d): %v", o.ID, err)
			} else {
				log.Printf("checker: fixed %s link type %s -> %s", o.Number, o.LinkType, actual)
				fixed++
			}
		}

		// Fix src_path: for symlinks, read the target; hardlinks can't be resolved
		if o.SrcPath == "" && actual == "symlink" {
			if target, err := os.Readlink(o.LinkPath); err == nil && target != "" {
				if err := s.SetOutputSrcPath(ctx, o.ID, target); err != nil {
					log.Printf("checker fix src_path(%d): %v", o.ID, err)
				} else {
					log.Printf("checker: fixed %s src_path -> %s", o.Number, target)
					fixed++
				}
			}
		}
	}
	if fixed > 0 {
		log.Printf("checker: fixed %d output records", fixed)
	}
}
