package provider

import (
	"context"
	"testing"
)

func TestDMMScrapeNotConfigured(t *testing.T) {
	p := NewDMM("", "")
	meta, err := p.Scrape(context.Background(), Predict{Number: "ACHJ-057"})
	if err == nil || meta != nil {
		t.Fatal("expected not configured error")
	}
}

func TestDMMToMetadata(t *testing.T) {
	p := NewDMM("x", "y")
	m := p.toMetadata("ACHJ-057", &dmmItem{
		Title:   "Title/Invalid",
		Date:    "2025-01-15 10:00:00",
		Volume: "120",
		ImageURL: dmmImageURL{
			Large: "https://img.example.com/l.jpg",
		},
		ItemInfo: dmmItemInfo{
			Actress:  []dmmNameID{{Name: "A"}},
			Genre:    []dmmNameID{{Name: "G"}},
			Maker:    []dmmNameID{{Name: "M"}},
			Director: []dmmNameID{{Name: "D"}},
			Label:    []dmmNameID{{Name: "L"}},
			Series:   []dmmNameID{{Name: "S"}},
		},
	})
	if m.Number != "ACHJ-057" || m.Premiered != "2025-01-15" || m.Title == "" {
		t.Fatalf("unexpected meta: %+v", m)
	}
}

func TestSelectDMMItemFiltersMismatchedContentID(t *testing.T) {
	items := []dmmItem{
		{ContentID: "ovvr558", ProductID: "ovvr558", Title: "Wrong DVD result"},
		{ContentID: "mdvr00242", ProductID: "mdvr00242", Title: "Correct monthly result"},
		{ContentID: "mdvr00242", ProductID: "mdvr00242", Title: "Correct digital result"},
	}

	item := selectDMMItem("mdvr00242", items)
	if item == nil {
		t.Fatal("expected a matched item")
	}
	if item.ContentID != "mdvr00242" {
		t.Fatalf("expected content_id mdvr00242, got %s", item.ContentID)
	}
	if item.Title != "Correct monthly result" {
		t.Fatalf("expected to keep first matched item, got %q", item.Title)
	}
}

func TestDMMItemMatchesKeywordWithPrefixedContentID(t *testing.T) {
	item := &dmmItem{ContentID: "h_1711tnvr00001", ProductID: "h_1711tnvr00001"}
	if !dmmItemMatches(buildDMMMatchKeys("TNVR-001"), item) {
		t.Fatal("expected prefixed content_id to match normalized keyword")
	}
}
