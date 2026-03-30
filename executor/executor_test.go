package executor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/CheerChen/bangou/provider"
)

func TestLinkFile(t *testing.T) {
	srcDir := t.TempDir()
	outDir := t.TempDir()

	srcPath := filepath.Join(srcDir, "test.mp4")
	if err := os.WriteFile(srcPath, []byte("video data"), 0o644); err != nil {
		t.Fatal(err)
	}

	targetDir := filepath.Join(outDir, "ACHJ-057")
	linkPath, err := LinkFile(srcPath, targetDir, "ACHJ-057", "hardlink", false, 0)
	if err != nil {
		t.Fatal(err)
	}

	expected := filepath.Join(targetDir, "ACHJ-057.mp4")
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

func TestResolveLinkPath(t *testing.T) {
	meta := &provider.MovieMetadata{
		Year:   "2026",
		Actors: []string{"村上悠華"},
	}
	tests := []struct {
		pattern string
		want    string
	}{
		{"", "SIVR-476"},
		{"{Number}", "SIVR-476"},
		{"{Year}/{Number}", "2026/SIVR-476"},
		{"{Year}/{Actor}/{Number}", "2026/村上悠華/SIVR-476"},
		{"{Year}/{Actor}", "2026/村上悠華/SIVR-476"}, // Number auto-appended
	}
	for _, tc := range tests {
		got := ResolveLinkPath(tc.pattern, "SIVR-476", meta)
		if got != tc.want {
			t.Errorf("ResolveLinkPath(%q) = %q, want %q", tc.pattern, got, tc.want)
		}
	}

	// nil meta → fallback to "Unknown"
	got := ResolveLinkPath("{Year}/{Actor}/{Number}", "SIVR-476", nil)
	if got != "Unknown/Unknown/SIVR-476" {
		t.Errorf("nil meta: got %q", got)
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
