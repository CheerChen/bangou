package backup

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/zeroAlcBeer/bangou/committed"
)

func TestOnceCreatesAndSkipsSameDay(t *testing.T) {
	tmp := t.TempDir()
	db, err := committed.NewSQLite(filepath.Join(tmp, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	dir := filepath.Join(tmp, "backups")
	if err := Once(context.Background(), db, dir); err != nil {
		t.Fatalf("first backup: %v", err)
	}
	files, _ := filepath.Glob(filepath.Join(dir, "bangou-*.db"))
	if len(files) != 1 {
		t.Fatalf("want 1 snapshot, got %d", len(files))
	}

	// Second run on the same day is a no-op.
	if err := Once(context.Background(), db, dir); err != nil {
		t.Fatalf("second backup: %v", err)
	}
	files, _ = filepath.Glob(filepath.Join(dir, "bangou-*.db"))
	if len(files) != 1 {
		t.Fatalf("want 1 snapshot after rerun, got %d", len(files))
	}

	// The snapshot is a valid database.
	snap, err := committed.NewSQLite(files[0])
	if err != nil {
		t.Fatalf("open snapshot: %v", err)
	}
	if _, err := snap.ListPipelines(context.Background()); err != nil {
		t.Fatalf("query snapshot: %v", err)
	}
	snap.Close()
}

func TestPruneKeepsNewest(t *testing.T) {
	dir := t.TempDir()
	names := []string{
		"bangou-20260101.db", "bangou-20260102.db", "bangou-20260103.db",
		"bangou-20260104.db", "bangou-20260105.db", "bangou-20260106.db",
		"bangou-20260107.db", "bangou-20260108.db", "bangou-20260109.db",
	}
	for _, n := range names {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := prune(dir); err != nil {
		t.Fatal(err)
	}
	files, _ := filepath.Glob(filepath.Join(dir, "bangou-*.db"))
	if len(files) != keepSnapshots {
		t.Fatalf("want %d files, got %d", keepSnapshots, len(files))
	}
	if _, err := os.Stat(filepath.Join(dir, "bangou-20260101.db")); !os.IsNotExist(err) {
		t.Fatal("oldest snapshot should be pruned")
	}
	if _, err := os.Stat(filepath.Join(dir, "bangou-20260109.db")); err != nil {
		t.Fatal("newest snapshot should be kept")
	}
}
