package provider

import (
	"os"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

func TestAVWikiParseDetailFixture(t *testing.T) {
	f, err := os.Open("testdata/avwiki_detail.html")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	doc, err := goquery.NewDocumentFromReader(f)
	if err != nil {
		t.Fatal(err)
	}

	p := NewAVWiki()
	meta := p.parseDetail(doc, "ACHJ-057")
	if meta == nil {
		t.Fatal("expected metadata")
	}
	if !strings.Contains(meta.Title, "Sample Title") {
		t.Fatalf("title: %s", meta.Title)
	}
	if meta.Maker != "Studio X" {
		t.Fatalf("maker: %s", meta.Maker)
	}
	if len(meta.Actors) != 2 {
		t.Fatalf("actors: %+v", meta.Actors)
	}
	if meta.CoverURL == "" {
		t.Fatal("cover url empty")
	}
}
