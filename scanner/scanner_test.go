package scanner

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanDir(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "ACHJ-057.mp4"), make([]byte, 1024), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ACHJ-057.mp4.aria2"), []byte("downloading"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "HMN-690.mp4"), make([]byte, 2048), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("not a video"), 0o644); err != nil {
		t.Fatal(err)
	}

	files, err := ScanDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("got %d files, want 2", len(files))
	}

	byName := map[string]ScannedFile{}
	for _, f := range files {
		byName[f.Filename] = f
	}

	if !byName["HMN-690.mp4"].Ready {
		t.Error("HMN-690 should be ready")
	}
	if byName["ACHJ-057.mp4"].Ready {
		t.Error("ACHJ-057 should not be ready")
	}
}
