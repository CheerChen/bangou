package web

import (
	"testing"

	"github.com/CheerChen/bangou/staging"
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

func TestIsGroupLinkEligible(t *testing.T) {
	makeGroup := func(status, task string, items []staging.StagedItem) staging.StagingGroup {
		return staging.StagingGroup{
			Scrape: staging.ScrapeResult{Status: status},
			Task:   task,
			Items:  items,
		}
	}

	t.Run("eligible", func(t *testing.T) {
		g := makeGroup("success", "", []staging.StagedItem{
			{File: staging.StagingFile{Path: "/a/x.mp4", Ready: true}},
			{File: staging.StagingFile{Path: "/a/y.mp4", Ready: true}},
		})
		if !isGroupLinkEligible(g) {
			t.Fatal("expected eligible group")
		}
	})

	t.Run("mixed extension not eligible", func(t *testing.T) {
		g := makeGroup("success", "", []staging.StagedItem{
			{File: staging.StagingFile{Path: "/a/x.mp4", Ready: true}},
			{File: staging.StagingFile{Path: "/a/y.mkv", Ready: true}},
		})
		if isGroupLinkEligible(g) {
			t.Fatal("expected mixed extension group to be ineligible")
		}
	})

	t.Run("not all ready not eligible", func(t *testing.T) {
		g := makeGroup("success", "", []staging.StagedItem{
			{File: staging.StagingFile{Path: "/a/x.mp4", Ready: true}},
			{File: staging.StagingFile{Path: "/a/y.mp4", Ready: false}},
		})
		if isGroupLinkEligible(g) {
			t.Fatal("expected not-ready group to be ineligible")
		}
	})
}
