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
		{ContentID: "ovvr558", ProductID: "ovvr558", ServiceCode: "digital", Title: "Wrong DVD result"},
		{ContentID: "mdvr00242", ProductID: "mdvr00242", ServiceCode: "monthly", Title: "Monthly result"},
		{ContentID: "mdvr00242", ProductID: "mdvr00242", ServiceCode: "digital", Title: "Digital result"},
	}

	item := selectDMMItem("mdvr00242", items)
	if item == nil {
		t.Fatal("expected a matched item")
	}
	if item.ContentID != "mdvr00242" {
		t.Fatalf("expected content_id mdvr00242, got %s", item.ContentID)
	}
	if item.Title != "Digital result" {
		t.Fatalf("expected digital preferred over monthly, got %q", item.Title)
	}
}

func TestSelectDMMItemPrefersDigitalOverMono(t *testing.T) {
	items := []dmmItem{
		{ContentID: "sone00408", ProductID: "sone00408", ServiceCode: "mono", Title: "DVD"},
		{ContentID: "sone00408", ProductID: "sone00408", ServiceCode: "digital", Title: "Digital"},
		{ContentID: "sone00408", ProductID: "sone00408", ServiceCode: "monthly", Title: "Monthly"},
		{ContentID: "sone408bod", ProductID: "sone408bod", ServiceCode: "mono", Title: "BOD"},
	}

	item := selectDMMItem("sone00408", items)
	if item == nil {
		t.Fatal("expected a matched item")
	}
	if item.Title != "Digital" {
		t.Fatalf("expected Digital, got %q", item.Title)
	}
}

func TestSelectDMMItemFallsBackToMono(t *testing.T) {
	items := []dmmItem{
		{ContentID: "abc00123", ProductID: "abc00123", ServiceCode: "mono", Title: "DVD only"},
	}
	item := selectDMMItem("abc00123", items)
	if item == nil || item.Title != "DVD only" {
		t.Fatalf("expected fallback to mono, got %v", item)
	}
}

func TestDMMGuessLargeCover(t *testing.T) {
	tests := []struct{ in, want string }{
		{"https://pics.dmm.co.jp/digital/video/dass00185/dass00185ps.jpg", "https://pics.dmm.co.jp/digital/video/dass00185/dass00185pl.jpg"},
		{"https://pics.dmm.co.jp/mono/movie/adult/dass00185/dass00185ps.jpg", "https://pics.dmm.co.jp/mono/movie/adult/dass00185/dass00185pl.jpg"},
		{"https://example.com/no-pattern.jpg", "https://example.com/no-pattern.jpg"},
		{"", ""},
	}
	for _, tc := range tests {
		got := dmmGuessLargeCover(tc.in)
		if got != tc.want {
			t.Errorf("dmmGuessLargeCover(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestDMMGuessLargeSamples(t *testing.T) {
	in := []string{
		"https://pics.dmm.co.jp/digital/video/dass00185/dass00185-1.jpg",
		"https://pics.dmm.co.jp/digital/video/dass00185/dass00185-10.jpg",
	}
	want := []string{
		"https://pics.dmm.co.jp/digital/video/dass00185/dass00185jp-1.jpg",
		"https://pics.dmm.co.jp/digital/video/dass00185/dass00185jp-10.jpg",
	}
	got := dmmGuessLargeSamples(in)
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("[%d] got %q, want %q", i, got[i], want[i])
		}
	}
}

func TestDMMItemMatchesKeywordWithPrefixedContentID(t *testing.T) {
	item := &dmmItem{ContentID: "h_1711tnvr00001", ProductID: "h_1711tnvr00001"}
	if !dmmItemMatches(buildDMMMatchKeys("TNVR-001"), item) {
		t.Fatal("expected prefixed content_id to match normalized keyword")
	}
}
