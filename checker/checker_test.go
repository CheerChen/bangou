package checker

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheckLink(t *testing.T) {
	dir := t.TempDir()
	existing := filepath.Join(dir, "exists.mp4")
	if err := os.WriteFile(existing, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !CheckLink(existing) {
		t.Fatal("existing file should report alive")
	}
	if CheckLink(filepath.Join(dir, "gone.mp4")) {
		t.Fatal("missing file should report not alive")
	}
}
