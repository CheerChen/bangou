package nfo

import (
	"strings"
	"testing"

	"github.com/zeroAlcBeer/bangou/provider"
)

func TestGenerate(t *testing.T) {
	meta := &provider.MovieMetadata{
		Number:    "ACHJ-057",
		Title:     "Test Movie Title",
		Director:  "Test Director",
		Maker:     "Test Studio",
		Label:     "Test Label",
		Actors:    []string{"Actor A", "Actor B"},
		Genres:    []string{"Genre1", "Genre2"},
		Premiered: "2025-01-15",
		Year:      "2025",
	}

	xml, err := Generate(meta)
	if err != nil {
		t.Fatal(err)
	}
	s := string(xml)
	if !strings.Contains(s, "<title>ACHJ-057 Test Movie Title</title>") {
		t.Error("title not found")
	}
	if !strings.Contains(s, "<name>Actor A</name>") {
		t.Error("actor not found")
	}
	if !strings.Contains(s, "<genre>Genre1</genre>") {
		t.Error("genre not found")
	}
	if !strings.Contains(s, "<studio>Test Studio</studio>") {
		t.Error("studio not found")
	}
	if !strings.Contains(s, "<?xml") {
		t.Error("xml header missing")
	}
}
