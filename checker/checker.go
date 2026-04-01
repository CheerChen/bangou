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
	fixBangouFileRecords(ctx, s)
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
	files, err := s.ListAllBangouFiles(ctx)
	if err != nil {
		log.Printf("checker list files: %v", err)
		return
	}
	for _, f := range files {
		alive := CheckLink(f.LinkPath)
		if alive == f.Alive {
			continue
		}
		if err := s.SetBangouFileAlive(ctx, f.ID, alive); err != nil {
			log.Printf("checker update file(%d): %v", f.ID, err)
			continue
		}
		if !alive {
			log.Printf("orphaned: file %d -> %s", f.ID, f.LinkPath)
		}
	}
}

func fixBangouFileRecords(ctx context.Context, s committed.Store) {
	files, err := s.ListAllBangouFiles(ctx)
	if err != nil {
		log.Printf("checker fix files: %v", err)
		return
	}
	fixed := 0
	for _, f := range files {
		actual := detectActualLinkType(f.LinkPath)
		if actual == "" {
			continue
		}
		if actual != f.LinkType {
			if err := s.SetBangouFileLinkType(ctx, f.ID, actual); err != nil {
				log.Printf("checker fix link type(%d): %v", f.ID, err)
			} else {
				log.Printf("checker: fixed file %d link type %s -> %s", f.ID, f.LinkType, actual)
				fixed++
			}
		}
		if f.SrcPath == "" && actual == "symlink" {
			if target, err := os.Readlink(f.LinkPath); err == nil && target != "" {
				if err := s.SetBangouFileSrcPath(ctx, f.ID, target); err != nil {
					log.Printf("checker fix src_path(%d): %v", f.ID, err)
				} else {
					log.Printf("checker: fixed file %d src_path -> %s", f.ID, target)
					fixed++
				}
			}
		}
	}
	if fixed > 0 {
		log.Printf("checker: fixed %d file records", fixed)
	}
}

func backfillMediaInfo(ctx context.Context, s committed.Store) {
	files, err := s.ListAllBangouFiles(ctx)
	if err != nil {
		log.Printf("checker backfill: %v", err)
		return
	}
	filled := 0
	for _, f := range files {
		if f.Resolution != "" {
			continue
		}
		if !f.Alive {
			continue
		}

		probePath := f.LinkPath
		media, err := scanner.Probe(probePath)
		if err != nil {
			log.Printf("checker backfill file %d: probe failed: %v", f.ID, err)
			continue
		}
		if media == nil {
			continue
		}

		fi, err := os.Stat(probePath)
		var fileSize int64
		if err == nil {
			fileSize = fi.Size()
		}
		if fileSize > 0 && media.Duration > 0 {
			media.BitrateBps = int64(float64(fileSize*8) / media.Duration)
		}

		if err := s.SetBangouFileMedia(ctx, f.ID, fileSize, media.Resolution(), media.VideoCodec, media.AudioCodec, media.DurationText(), media.BitrateText()); err != nil {
			log.Printf("checker backfill file %d: update failed: %v", f.ID, err)
			continue
		}
		log.Printf("checker: backfilled file %d media: %s %s %s", f.ID, media.Resolution(), media.VideoCodec, media.BitrateText())
		filled++
	}
	if filled > 0 {
		log.Printf("checker: backfilled media info for %d files", filled)
	}
}
