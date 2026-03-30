package provider

import (
	"context"
	"testing"
)

func TestDMMScrapeNotConfigured(t *testing.T) {
	p := NewDMM("", "")
	meta, err := p.Scrape(context.Background(), "ACHJ-057")
	if err == nil || meta != nil {
		t.Fatal("expected not configured error")
	}
}

func TestDMMToMetadata(t *testing.T) {
	p := NewDMM("x", "y")
	m := p.toMetadata("ACHJ-057", &dmmItem{
		Title:   "Title/Invalid",
		Date:    "2025-01-15 10:00:00",
		Runtime: "120",
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
