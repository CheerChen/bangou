package provider

import (
	"context"
	"errors"
	"testing"
)

type fakeProvider struct {
	name string
	meta *MovieMetadata
	err  error
}

func (f fakeProvider) Name() string { return f.name }
func (f fakeProvider) Scrape(ctx context.Context, p Predict) (*MovieMetadata, error) {
	_ = ctx
	_ = p
	return f.meta, f.err
}

func TestScrapeAll(t *testing.T) {
	res := ScrapeAll(context.Background(), []Provider{
		fakeProvider{name: "a", err: errors.New("failed")},
		fakeProvider{name: "b", meta: &MovieMetadata{Number: "ABC-001", Title: "ok"}},
	}, Predict{Number: "ABC-001"}, nil)
	if res.Meta == nil {
		t.Fatal("expected metadata")
	}
	if res.Meta.Provider != "b" {
		t.Fatalf("provider = %s", res.Meta.Provider)
	}
	if res.Errors["a"] == "" {
		t.Fatal("expected error for provider a")
	}
}

func TestScrapeAllMerge(t *testing.T) {
	var phase1 *ScrapeResult
	res := ScrapeAll(context.Background(), []Provider{
		fakeProvider{name: "dmm", meta: &MovieMetadata{Number: "ABC-001", Title: "dmm title", Premiered: "2026-01-01"}},
		fakeProvider{name: "avwiki", meta: &MovieMetadata{Number: "ABC-001", Title: "avwiki title", Actors: []string{"A"}, Premiered: "2025-12-25"}},
	}, Predict{Number: "ABC-001"}, func(first *ScrapeResult) {
		phase1 = first
	})
	if phase1 == nil || phase1.Meta == nil {
		t.Fatal("expected phase1 callback")
	}
	if res.Meta == nil {
		t.Fatal("expected final metadata")
	}
	// Rule 1: actors filled (dmm has none, avwiki has)
	if len(res.Meta.Actors) != 1 || res.Meta.Actors[0] != "A" {
		t.Fatalf("actors = %v", res.Meta.Actors)
	}
	// Rule 2: avwiki date preferred over dmm date
	if res.Meta.Premiered != "2025-12-25" {
		t.Fatalf("premiered = %s, want 2025-12-25", res.Meta.Premiered)
	}
}

func TestSplitNumber(t *testing.T) {
	label, num, err := splitNumber("ACHJ-057")
	if err != nil {
		t.Fatal(err)
	}
	if label != "ACHJ" || num != "057" {
		t.Fatalf("got %s %s", label, num)
	}
}
