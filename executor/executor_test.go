package executor

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/zeroAlcBeer/bangou/committed"
	"github.com/zeroAlcBeer/bangou/provider"
	"github.com/zeroAlcBeer/bangou/staging"
)

func TestLinkFile(t *testing.T) {
	srcDir := t.TempDir()
	outDir := t.TempDir()

	srcPath := filepath.Join(srcDir, "test.mp4")
	if err := os.WriteFile(srcPath, []byte("video data"), 0o644); err != nil {
		t.Fatal(err)
	}

	targetDir := filepath.Join(outDir, "ACHJ-057")
	result, err := LinkFile(srcPath, targetDir, "ACHJ-057", "hardlink", false, 0)
	if err != nil {
		t.Fatal(err)
	}

	expected := filepath.Join(targetDir, "ACHJ-057.mp4")
	if result.LinkPath != expected {
		t.Fatalf("linkPath = %q, want %q", result.LinkPath, expected)
	}
	if result.LinkType != "hardlink" {
		t.Fatalf("linkType = %q, want %q", result.LinkType, "hardlink")
	}

	data, err := os.ReadFile(result.LinkPath)
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

func TestMergeProgressFromSize(t *testing.T) {
	tests := []struct {
		name        string
		currentSize int64
		totalSize   int64
		want        int
	}{
		{name: "zero total", currentSize: 50, totalSize: 0, want: 0},
		{name: "zero current", currentSize: 0, totalSize: 100, want: 0},
		{name: "half", currentSize: 50, totalSize: 100, want: 50},
		{name: "full still capped", currentSize: 100, totalSize: 100, want: 99},
		{name: "overflow capped", currentSize: 120, totalSize: 100, want: 99},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := mergeProgressFromSize(tc.currentSize, tc.totalSize)
			if got != tc.want {
				t.Fatalf("mergeProgressFromSize(%d, %d) = %d, want %d", tc.currentSize, tc.totalSize, got, tc.want)
			}
		})
	}
}

func TestBuildMergeSourceTag(t *testing.T) {
	t.Run("all without part", func(t *testing.T) {
		selected := []staging.StagedItem{
			{Parsed: staging.ParsedFile{Part: 0}},
			{Parsed: staging.ParsedFile{Part: 0}},
			{Parsed: staging.ParsedFile{Part: 0}},
		}
		got, err := buildMergeSourceTag(selected)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if got != "m123" {
			t.Fatalf("tag = %q, want %q", got, "m123")
		}
	})

	t.Run("all with part", func(t *testing.T) {
		selected := []staging.StagedItem{
			{Parsed: staging.ParsedFile{Part: 1}},
			{Parsed: staging.ParsedFile{Part: 3}},
		}
		got, err := buildMergeSourceTag(selected)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if got != "m13" {
			t.Fatalf("tag = %q, want %q", got, "m13")
		}
	})

	t.Run("mixed part should fail", func(t *testing.T) {
		selected := []staging.StagedItem{
			{Parsed: staging.ParsedFile{Part: 2}},
			{Parsed: staging.ParsedFile{Part: 0}},
		}
		if _, err := buildMergeSourceTag(selected); err == nil {
			t.Fatal("expected error for mixed part selection")
		}
	})
}

func TestValidateSingleExtensionSelection(t *testing.T) {
	t.Run("single extension", func(t *testing.T) {
		selected := []staging.StagedItem{
			{File: staging.StagingFile{Filename: "a.mp4"}},
			{File: staging.StagingFile{Filename: "b.mp4"}},
		}
		if err := validateSingleExtensionSelection(selected); err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
	})

	t.Run("mixed extension", func(t *testing.T) {
		selected := []staging.StagedItem{
			{File: staging.StagingFile{Filename: "a.mp4"}},
			{File: staging.StagingFile{Filename: "b.mkv"}},
		}
		if err := validateSingleExtensionSelection(selected); err == nil {
			t.Fatal("expected error for mixed extensions")
		}
	})
}

func TestHasMixedMKVAndMP4(t *testing.T) {
	items := []staging.StagedItem{
		{File: staging.StagingFile{Filename: "a.mp4"}},
		{File: staging.StagingFile{Filename: "b.mkv"}},
	}
	if !hasMixedMKVAndMP4(items) {
		t.Fatal("expected mixed mkv/mp4 to be true")
	}

	items = []staging.StagedItem{
		{File: staging.StagingFile{Filename: "a.mp4"}},
		{File: staging.StagingFile{Filename: "b.avi"}},
	}
	if hasMixedMKVAndMP4(items) {
		t.Fatal("expected mixed mkv/mp4 to be false")
	}
}

func TestUnlinkRemovesBangouFile(t *testing.T) {
	ctx := context.Background()
	store, err := committed.NewSQLite(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	outputDir := t.TempDir()
	pid, err := store.CreatePipeline(ctx, &committed.Pipeline{
		Name: "VR", InputDir: t.TempDir(), OutputDir: outputDir,
	})
	if err != nil {
		t.Fatal(err)
	}

	linkDir := filepath.Join(t.TempDir(), "custom", "path")
	if err := os.MkdirAll(linkDir, 0o755); err != nil {
		t.Fatal(err)
	}
	linkPath := filepath.Join(linkDir, "URVRSP-229-cd1.mp4")
	if err := os.WriteFile(linkPath, []byte("linked"), 0o644); err != nil {
		t.Fatal(err)
	}

	bid, err := store.CreateBangou(ctx, &committed.Bangou{
		PipelineID: pid, Number: "URVRSP-229", OutDir: linkDir,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CreateBangouFile(ctx, &committed.BangouFile{
		BangouID: bid, LinkPath: linkPath, LinkType: "symlink",
	}); err != nil {
		t.Fatal(err)
	}

	files, err := store.ListBangouFilesByBangou(ctx, bid)
	if err != nil || len(files) != 1 {
		t.Fatalf("list files: err=%v len=%d", err, len(files))
	}

	exec := New(store, staging.New(), outputDir)
	if err := exec.Unlink(ctx, &files[0]); err != nil {
		t.Fatalf("unlink: %v", err)
	}

	// Link file should be removed
	if _, err := os.Stat(linkPath); !os.IsNotExist(err) {
		t.Fatalf("expected link file removed, stat err=%v", err)
	}

	// BangouFile record should be gone
	remaining, _ := store.CountBangouFiles(ctx, bid)
	if remaining != 0 {
		t.Fatalf("expected 0 remaining files, got %d", remaining)
	}

	// Bangou itself should be deleted (last file was removed)
	b, _ := store.GetBangou(ctx, bid)
	if b != nil {
		t.Fatal("expected bangou record removed after last file unlinked")
	}
}

func TestUnlinkKeepsBangouWhenFilesRemain(t *testing.T) {
	ctx := context.Background()
	store, err := committed.NewSQLite(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	outputDir := t.TempDir()
	pid, _ := store.CreatePipeline(ctx, &committed.Pipeline{
		Name: "VR", InputDir: t.TempDir(), OutputDir: outputDir,
	})

	linkDir := filepath.Join(t.TempDir(), "out", "SIVR-296")
	os.MkdirAll(linkDir, 0o755)
	link1 := filepath.Join(linkDir, "SIVR-296-cd1.mp4")
	link2 := filepath.Join(linkDir, "SIVR-296-cd2.mp4")
	os.WriteFile(link1, []byte("part1"), 0o644)
	os.WriteFile(link2, []byte("part2"), 0o644)

	bid, _ := store.CreateBangou(ctx, &committed.Bangou{
		PipelineID: pid, Number: "SIVR-296", OutDir: linkDir,
	})
	store.CreateBangouFile(ctx, &committed.BangouFile{BangouID: bid, LinkPath: link1, LinkType: "hardlink"})
	store.CreateBangouFile(ctx, &committed.BangouFile{BangouID: bid, LinkPath: link2, LinkType: "hardlink"})

	files, _ := store.ListBangouFilesByBangou(ctx, bid)
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}

	// Unlink only the first file
	exec := New(store, staging.New(), outputDir)
	if err := exec.Unlink(ctx, &files[0]); err != nil {
		t.Fatalf("unlink: %v", err)
	}

	// Bangou should still exist
	b, _ := store.GetBangou(ctx, bid)
	if b == nil {
		t.Fatal("expected bangou to still exist with 1 remaining file")
	}

	remaining, _ := store.CountBangouFiles(ctx, bid)
	if remaining != 1 {
		t.Fatalf("expected 1 remaining file, got %d", remaining)
	}
}

func TestRestoreLinkRecreatesMissingBangouFile(t *testing.T) {
	ctx := context.Background()
	store, err := committed.NewSQLite(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	outputDir := t.TempDir()
	pid, _ := store.CreatePipeline(ctx, &committed.Pipeline{
		Name: "VR", InputDir: t.TempDir(), OutputDir: outputDir,
	})

	srcDir := t.TempDir()
	srcPath := filepath.Join(srcDir, "ACHJ-057.mp4")
	if err := os.WriteFile(srcPath, []byte("source"), 0o644); err != nil {
		t.Fatal(err)
	}

	linkDir := filepath.Join(outputDir, "ACHJ-057")
	linkPath := filepath.Join(linkDir, "ACHJ-057.mp4")
	bid, _ := store.CreateBangou(ctx, &committed.Bangou{
		PipelineID: pid, Number: "ACHJ-057", OutDir: linkDir,
	})
	if err := store.CreateBangouFile(ctx, &committed.BangouFile{
		BangouID: bid, SrcPath: srcPath, LinkPath: linkPath, LinkType: "hardlink",
	}); err != nil {
		t.Fatal(err)
	}
	files, _ := store.ListBangouFilesByBangou(ctx, bid)
	if err := store.SetBangouFileAlive(ctx, files[0].ID, false); err != nil {
		t.Fatal(err)
	}
	files, _ = store.ListBangouFilesByBangou(ctx, bid)

	exec := New(store, staging.New(), outputDir)
	if err := exec.RestoreLink(ctx, &files[0]); err != nil {
		t.Fatalf("restore: %v", err)
	}
	if data, err := os.ReadFile(linkPath); err != nil || string(data) != "source" {
		t.Fatalf("restored link data=%q err=%v", data, err)
	}
	files, _ = store.ListBangouFilesByBangou(ctx, bid)
	if !files[0].Alive {
		t.Fatal("expected restored file marked alive")
	}
}

func TestUnlinkCleansUpArtifacts(t *testing.T) {
	ctx := context.Background()
	store, err := committed.NewSQLite(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	outputDir := t.TempDir()
	pid, _ := store.CreatePipeline(ctx, &committed.Pipeline{
		Name: "VR", InputDir: t.TempDir(), OutputDir: outputDir,
	})

	linkDir := filepath.Join(t.TempDir(), "out", "ACHJ-057")
	os.MkdirAll(linkDir, 0o755)

	// Create link file + metadata artifacts
	linkPath := filepath.Join(linkDir, "ACHJ-057.mp4")
	nfoPath := filepath.Join(linkDir, "ACHJ-057.nfo")
	coverPath := filepath.Join(linkDir, "ACHJ-057.jpg")
	rawPath := filepath.Join(linkDir, "ACHJ-057-raw.dmm")
	for _, p := range []string{linkPath, nfoPath, coverPath, rawPath} {
		os.WriteFile(p, []byte("data"), 0o644)
	}

	bid, _ := store.CreateBangou(ctx, &committed.Bangou{
		PipelineID: pid, Number: "ACHJ-057", OutDir: linkDir,
	})
	store.UpdateBangouPaths(ctx, bid, nfoPath, coverPath, rawPath)
	store.CreateBangouFile(ctx, &committed.BangouFile{BangouID: bid, LinkPath: linkPath, LinkType: "hardlink"})

	files, _ := store.ListBangouFilesByBangou(ctx, bid)

	exec := New(store, staging.New(), outputDir)
	if err := exec.Unlink(ctx, &files[0]); err != nil {
		t.Fatalf("unlink: %v", err)
	}

	// All artifacts should be removed
	for _, p := range []string{linkPath, nfoPath, coverPath, rawPath} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Fatalf("expected %s removed", filepath.Base(p))
		}
	}

	// Output directory should be removed (was empty after cleanup)
	if _, err := os.Stat(linkDir); !os.IsNotExist(err) {
		t.Fatalf("expected output directory removed")
	}
}
