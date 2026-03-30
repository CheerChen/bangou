package executor

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLinkFile(t *testing.T) {
	srcDir := t.TempDir()
	outDir := t.TempDir()

	srcPath := filepath.Join(srcDir, "test.mp4")
	if err := os.WriteFile(srcPath, []byte("video data"), 0o644); err != nil {
		t.Fatal(err)
	}

	linkPath, err := LinkFile(srcPath, outDir, "ACHJ-057", "hardlink", false, 0)
	if err != nil {
		t.Fatal(err)
	}

	expected := filepath.Join(outDir, "ACHJ-057", "ACHJ-057.mp4")
	if linkPath != expected {
		t.Fatalf("linkPath = %q, want %q", linkPath, expected)
	}

	data, err := os.ReadFile(linkPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "video data" {
		t.Fatal("link content mismatch")
	}
}

func TestBuildMergeCommand(t *testing.T) {
	cmd := BuildMergeCommand([]string{"/tmp/a.mp4", "/tmp/b.mp4"}, "/tmp/out.mkv")
	if cmd.Path == "" {
		t.Fatal("expected command path")
	}
	if len(cmd.Args) < 5 {
		t.Fatalf("unexpected args: %v", cmd.Args)
	}
}
