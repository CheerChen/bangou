package staging

import "testing"

func TestIngestAndGroup(t *testing.T) {
	m := New()
	m.Ingest(StagingFile{Path: "/input/ACHJ-057.mp4", Filename: "ACHJ-057.mp4", Size: 1024, Ready: true})
	groups := m.ListGroups()
	if len(groups) != 1 {
		t.Fatalf("got %d groups, want 1", len(groups))
	}
	if groups[0].Number != "ACHJ-057" {
		t.Fatalf("number = %q", groups[0].Number)
	}
}

func TestIngestMultipleParts(t *testing.T) {
	m := New()
	m.Ingest(StagingFile{Path: "/input/sivr476_1_8k.mp4", Filename: "sivr476_1_8k.mp4", Size: 5000, Ready: true})
	m.Ingest(StagingFile{Path: "/input/sivr476_3_8k.mp4", Filename: "sivr476_3_8k.mp4", Size: 5000, Ready: true})
	groups := m.ListGroups()
	if len(groups) != 1 {
		t.Fatalf("got %d groups, want 1", len(groups))
	}
	if len(groups[0].Items) != 2 {
		t.Fatalf("got %d items", len(groups[0].Items))
	}
}

func TestIngestUnknown(t *testing.T) {
	m := New()
	m.Ingest(StagingFile{Path: "/input/random.mp4", Filename: "random.mp4", Size: 100, Ready: true})
	if len(m.ListGroups()) != 0 {
		t.Fatal("expected no groups")
	}
	if len(m.ListUnknowns()) != 1 {
		t.Fatal("expected 1 unknown")
	}
}

func TestRemoveFile(t *testing.T) {
	m := New()
	m.Ingest(StagingFile{Path: "/input/ACHJ-057.mp4", Filename: "ACHJ-057.mp4", Size: 1024, Ready: true})
	m.Remove("/input/ACHJ-057.mp4")
	if len(m.ListGroups()) != 0 {
		t.Fatal("group should be removed")
	}
}

func TestReconcile(t *testing.T) {
	m := New()
	m.Ingest(StagingFile{Path: "/input/a.mp4", Filename: "ACHJ-057.mp4", Size: 1, Ready: true})
	m.Reconcile(map[string]bool{})
	if len(m.ListGroups()) != 0 {
		t.Fatal("expected empty after reconcile")
	}
}

func TestSetDownloadProgressMatchesExactFilename(t *testing.T) {
	m := New()
	m.Ingest(StagingFile{
		Path:     "/downloads/VR/sivr00296-2.mp4",
		Filename: "sivr00296-2.mp4",
		Ready:    false,
	})

	matched := m.SetDownloadProgress("/downloads/VR/sivr00296-2.mp4", 47, 100, "active")
	if matched != "sivr00296-2.mp4" {
		t.Fatalf("matched filename = %q", matched)
	}

	g := m.GetGroup("SIVR-296")
	if g == nil || len(g.Items) != 1 {
		t.Fatal("expected group SIVR-296")
	}
	if got := g.Items[0].File.DownloadPct; got != 47 {
		t.Fatalf("download pct = %d, want 47", got)
	}
}

func TestSetDownloadProgressMatchesParsedNumberAndPart(t *testing.T) {
	m := New()
	m.Ingest(StagingFile{
		Path:     "/downloads/VR/twojav.com@sivr00296.part2.mp4",
		Filename: "twojav.com@sivr00296.part2.mp4",
		Ready:    false,
	})

	matched := m.SetDownloadProgress("/hd1downloads/VR/sivr00296-2.mp4", 47, 3616620544, "active")
	if matched != "twojav.com@sivr00296.part2.mp4" {
		t.Fatalf("matched filename = %q", matched)
	}

	g := m.GetGroup("SIVR-296")
	if g == nil || len(g.Items) != 1 {
		t.Fatal("expected group SIVR-296")
	}
	file := g.Items[0].File
	if file.DownloadPct != 47 {
		t.Fatalf("download pct = %d, want 47", file.DownloadPct)
	}
	if file.DownloadStatus != "active" {
		t.Fatalf("download status = %q, want active", file.DownloadStatus)
	}
}

func TestResolveDownloadReturnsEmptyOnAmbiguousPartlessMatch(t *testing.T) {
	m := New()
	m.Ingest(StagingFile{
		Path:     "/downloads/VR/sivr00296.part1.mp4",
		Filename: "sivr00296.part1.mp4",
		Ready:    false,
	})
	m.Ingest(StagingFile{
		Path:     "/downloads/VR/sivr00296.part2.mp4",
		Filename: "sivr00296.part2.mp4",
		Ready:    false,
	})

	if matched := m.ResolveDownload("/aria2/VR/sivr00296.mp4"); matched != "" {
		t.Fatalf("expected ambiguous download to remain unmatched, got %q", matched)
	}
}
