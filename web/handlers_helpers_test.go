package web

import (
	"testing"

	"github.com/zeroAlcBeer/bangou/staging"
)

func TestValidateSingleExtensionPaths(t *testing.T) {
	if err := validateSingleExtensionPaths([]string{"/a/x.mp4", "/a/y.mp4"}); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if err := validateSingleExtensionPaths([]string{"/a/x.mp4", "/a/y.mkv"}); err == nil {
		t.Fatal("expected mixed extension error")
	}
	if err := validateSingleExtensionPaths([]string{"/a/x"}); err == nil {
		t.Fatal("expected missing extension error")
	}
}

func TestHasMixedMKVAndMP4Group(t *testing.T) {
	items := []staging.StagedItem{
		{File: staging.StagingFile{Filename: "x.mp4"}},
		{File: staging.StagingFile{Filename: "y.mkv"}},
	}
	if !hasMixedMKVAndMP4Group(items) {
		t.Fatal("expected true")
	}

	items = []staging.StagedItem{
		{File: staging.StagingFile{Filename: "x.mp4"}},
		{File: staging.StagingFile{Filename: "y.avi"}},
	}
	if hasMixedMKVAndMP4Group(items) {
		t.Fatal("expected false")
	}
}

