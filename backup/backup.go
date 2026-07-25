package backup

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/zeroAlcBeer/bangou/committed"
)

const keepSnapshots = 7

// Run takes a daily snapshot of the database into dir, keeping the most
// recent 7. It checks hourly but skips days that already have a snapshot,
// so restarts never create duplicates.
func Run(ctx context.Context, db *committed.SQLiteStore, dir string) {
	once := func() {
		if err := Once(ctx, db, dir); err != nil {
			log.Printf("[backup] %v", err)
		}
	}
	once()

	t := time.NewTicker(time.Hour)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			once()
		}
	}
}

// Once writes today's snapshot if it does not exist yet, then prunes old ones.
func Once(ctx context.Context, db *committed.SQLiteStore, dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}
	dest := filepath.Join(dir, "bangou-"+time.Now().Format("20060102")+".db")
	if _, err := os.Stat(dest); err == nil {
		return nil // today's snapshot already exists
	}
	if err := db.VacuumInto(ctx, dest); err != nil {
		_ = os.Remove(dest)
		return fmt.Errorf("vacuum into %s: %w", dest, err)
	}
	log.Printf("[backup] wrote %s", dest)
	return prune(dir)
}

func prune(dir string) error {
	matches, err := filepath.Glob(filepath.Join(dir, "bangou-*.db"))
	if err != nil {
		return err
	}
	if len(matches) <= keepSnapshots {
		return nil
	}
	// Date-named files sort chronologically.
	sort.Strings(matches)
	for _, old := range matches[:len(matches)-keepSnapshots] {
		if err := os.Remove(old); err != nil {
			log.Printf("[backup] prune %s: %v", old, err)
		} else {
			log.Printf("[backup] pruned %s", filepath.Base(old))
		}
	}
	return nil
}
