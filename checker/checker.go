package checker

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/CheerChen/bangou/committed"
	"github.com/CheerChen/bangou/scanner"
)

func CheckLink(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

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
	fixOutputRecords(ctx, s)
	backfillMediaInfo(ctx, s)

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
		if actual != o.LinkType {
			if err := s.SetOutputLinkType(ctx, o.ID, actual); err != nil {
				log.Printf("checker fix link type(%d): %v", o.ID, err)
			} else {
				log.Printf("checker: fixed %s link type %s -> %s", o.Number, o.LinkType, actual)
				fixed++
			}
		}
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

// backfillMediaInfo probes files for outputs missing media info and updates the DB.
func backfillMediaInfo(ctx context.Context, s committed.Store) {
	outputs, err := s.ListAllOutputs(ctx)
	if err != nil {
		log.Printf("checker backfill: %v", err)
		return
	}
	filled := 0
	for _, o := range outputs {
		if o.Resolution != "" {
			continue // already has media info
		}
		if !o.Alive {
			continue // file missing, can't probe
		}

		// Probe the actual file (follow symlinks via link_path)
		probePath := o.LinkPath
		media, err := scanner.Probe(probePath)
		if err != nil {
			log.Printf("checker backfill %s: probe failed: %v", o.Number, err)
			continue
		}
		if media == nil {
			continue
		}

		// Get file size
		fi, err := os.Stat(probePath)
		var fileSize int64
		if err == nil {
			fileSize = fi.Size()
		}
		if fileSize > 0 && media.Duration > 0 {
			media.BitrateBps = int64(float64(fileSize*8) / media.Duration)
		}

		if err := s.SetOutputMedia(ctx, o.ID, fileSize, media.Resolution(), media.VideoCodec, media.AudioCodec, media.DurationText(), media.BitrateText()); err != nil {
			log.Printf("checker backfill %s: update failed: %v", o.Number, err)
			continue
		}
		log.Printf("checker: backfilled %s media: %s %s %s", o.Number, media.Resolution(), media.VideoCodec, media.BitrateText())
		filled++
	}
	if filled > 0 {
		log.Printf("checker: backfilled media info for %d outputs", filled)
	}
}
