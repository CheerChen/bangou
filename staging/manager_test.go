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
