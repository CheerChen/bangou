package store

import (
	"context"
	"testing"
)

func testStore(t *testing.T) Store {
	t.Helper()
	s, err := NewSQLite(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestUpsertAndGetFile(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	f := &SourceFile{Path: "/input/test.mp4", Filename: "test.mp4", Size: 1024}
	if err := s.UpsertFile(ctx, f); err != nil {
		t.Fatal(err)
	}

	got, err := s.GetFileByPath(ctx, "/input/test.mp4")
	if err != nil {
		t.Fatal(err)
	}
	if got.Filename != "test.mp4" || got.Size != 1024 || got.Ready {
		t.Errorf("unexpected file: %+v", got)
	}
}

func TestEnsureGroupIdempotent(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	g1, err := s.EnsureGroup(ctx, "SIVR-476")
	if err != nil {
		t.Fatal(err)
	}
	g2, err := s.EnsureGroup(ctx, "SIVR-476")
	if err != nil {
		t.Fatal(err)
	}
	if g1.ID != g2.ID {
		t.Errorf("EnsureGroup not idempotent: %d != %d", g1.ID, g2.ID)
	}
}

func TestUpsertParsedUpdates(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	if err := s.UpsertFile(ctx, &SourceFile{Path: "/input/a.mp4", Filename: "a.mp4", Size: 1, Ready: true}); err != nil {
		t.Fatal(err)
	}
	f, err := s.GetFileByPath(ctx, "/input/a.mp4")
	if err != nil {
		t.Fatal(err)
	}

	if err := s.UpsertParsed(ctx, &ParsedInfo{FileID: f.ID, Number: "ABC-123", Part: 1, Manual: false}); err != nil {
		t.Fatal(err)
	}
	if err := s.UpsertParsed(ctx, &ParsedInfo{FileID: f.ID, Number: "ABC-124", Part: 2, Manual: true}); err != nil {
		t.Fatal(err)
	}

	p, err := s.GetParsedByFileID(ctx, f.ID)
	if err != nil {
		t.Fatal(err)
	}
	if p.Number != "ABC-124" || p.Part != 2 || !p.Manual {
		t.Fatalf("unexpected parsed_info after update: %+v", p)
	}
}

func TestListUnknownRespectsIgnore(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	if err := s.UpsertFile(ctx, &SourceFile{Path: "/input/a.mp4", Filename: "a.mp4", Size: 1, Ready: true}); err != nil {
		t.Fatal(err)
	}
	if err := s.UpsertFile(ctx, &SourceFile{Path: "/input/b.mp4", Filename: "b.mp4", Size: 1, Ready: true}); err != nil {
		t.Fatal(err)
	}
	b, err := s.GetFileByPath(ctx, "/input/b.mp4")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetFileIgnored(ctx, b.ID); err != nil {
		t.Fatal(err)
	}

	unknown, err := s.ListUnknownFiles(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(unknown) != 1 || unknown[0].Filename != "a.mp4" {
		t.Fatalf("unexpected unknown list: %+v", unknown)
	}
}

func TestMetadataUpsertAndStatus(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	if err := s.UpsertMetadata(ctx, &Metadata{
		Number:       "ACHJ-057",
		Title:        "Title",
		ScrapeStatus: "success",
	}); err != nil {
		t.Fatal(err)
	}
	m, err := s.GetMetadata(ctx, "ACHJ-057")
	if err != nil || m == nil {
		t.Fatalf("metadata not found: %+v err=%v", m, err)
	}
	if m.Title != "Title" || m.ScrapeStatus != "success" {
		t.Fatalf("unexpected metadata: %+v", m)
	}

	if err := s.SetMetadataStatus(ctx, "ACHJ-057", "failed", "{\"avwiki\":\"404\"}"); err != nil {
		t.Fatal(err)
	}
	m, err = s.GetMetadata(ctx, "ACHJ-057")
	if err != nil {
		t.Fatal(err)
	}
	if m.ScrapeStatus != "failed" || m.ScrapeErrors == "" {
		t.Fatalf("status update failed: %+v", m)
	}
}
