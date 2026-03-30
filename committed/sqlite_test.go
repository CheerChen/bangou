package committed

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

func TestOutputsAndCommitted(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	if err := s.CreateOutput(ctx, &Output{Number: "ACHJ-057", LinkPath: "/out/a.mp4", LinkType: "hardlink"}); err != nil {
		t.Fatal(err)
	}
	ok, err := s.IsCommitted(ctx, "ACHJ-057")
	if err != nil || !ok {
		t.Fatalf("is committed: %v %v", ok, err)
	}
	outs, err := s.ListOutputsByNumber(ctx, "ACHJ-057")
	if err != nil || len(outs) != 1 {
		t.Fatalf("outputs: %v len=%d", err, len(outs))
	}
}

func TestSettingsAndMetadata(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	if err := s.SetSetting(ctx, "input_dir", "/input"); err != nil {
		t.Fatal(err)
	}
	v, err := s.GetSetting(ctx, "input_dir")
	if err != nil || v != "/input" {
		t.Fatalf("setting: %q err=%v", v, err)
	}
	if err := s.UpsertMetadata(ctx, &Metadata{Number: "ACHJ-057", Title: "T"}); err != nil {
		t.Fatal(err)
	}
	m, err := s.GetMetadata(ctx, "ACHJ-057")
	if err != nil || m == nil || m.Title != "T" {
		t.Fatalf("metadata: %+v err=%v", m, err)
	}
}
