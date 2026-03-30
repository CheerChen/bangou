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
func (f fakeProvider) Scrape(ctx context.Context, number string) (*MovieMetadata, error) {
	_ = ctx
	_ = number
	return f.meta, f.err
}

func TestChain(t *testing.T) {
	res := Chain(context.Background(), []Provider{
		fakeProvider{name: "a", err: errors.New("failed")},
		fakeProvider{name: "b", meta: &MovieMetadata{Number: "ABC-001", Title: "ok"}},
	}, "ABC-001")
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

func TestSplitNumber(t *testing.T) {
	label, num, err := splitNumber("ACHJ-057")
	if err != nil {
		t.Fatal(err)
	}
	if label != "ACHJ" || num != "057" {
		t.Fatalf("got %s %s", label, num)
	}
}
