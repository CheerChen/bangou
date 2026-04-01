package web

import (
	"testing"

	"github.com/CheerChen/bangou/committed"
	"github.com/CheerChen/bangou/scanner"
	"github.com/CheerChen/bangou/staging"
)

func TestDistributeDownloadProgressMatchesSinglePipelineByParsedIdentity(t *testing.T) {
	rt := &PipelineRuntime{
		Pipeline: committed.Pipeline{Name: "vr-a"},
		Manager:  staging.New(),
	}
	rt.Manager.Ingest(staging.StagingFile{
		Path:     "/downloads/VR/twojav.com@sivr00296.part2.mp4",
		Filename: "twojav.com@sivr00296.part2.mp4",
		Ready:    false,
	})

	distributeDownloadProgress([]*PipelineRuntime{rt}, []scanner.DownloadProgress{
		{Path: "/hd1downloads/VR/sivr00296-2.mp4", Pct: 47, Completed: 3616620544, Status: "active"},
	})

	group := rt.Manager.GetGroup("SIVR-296")
	if group == nil || len(group.Items) != 1 {
		t.Fatal("expected group SIVR-296")
	}
	if got := group.Items[0].File.DownloadPct; got != 47 {
		t.Fatalf("download pct = %d, want 47", got)
	}
}

func TestDistributeDownloadProgressSkipsAmbiguousPipelineMatch(t *testing.T) {
	rtA := &PipelineRuntime{
		Pipeline: committed.Pipeline{Name: "vr-a"},
		Manager:  staging.New(),
	}
	rtB := &PipelineRuntime{
		Pipeline: committed.Pipeline{Name: "vr-b"},
		Manager:  staging.New(),
	}

	for _, rt := range []*PipelineRuntime{rtA, rtB} {
		rt.Manager.Ingest(staging.StagingFile{
			Path:     "/downloads/VR/sivr00296-2.mp4",
			Filename: "sivr00296-2.mp4",
			Ready:    false,
		})
	}

	distributeDownloadProgress([]*PipelineRuntime{rtA, rtB}, []scanner.DownloadProgress{
		{Path: "/hd1downloads/VR/sivr00296-2.mp4", Pct: 47, Completed: 3616620544, Status: "active"},
	})

	for _, rt := range []*PipelineRuntime{rtA, rtB} {
		group := rt.Manager.GetGroup("SIVR-296")
		if group == nil || len(group.Items) != 1 {
			t.Fatalf("expected group SIVR-296 for pipeline %s", rt.Pipeline.Name)
		}
		if got := group.Items[0].File.DownloadPct; got != 0 {
			t.Fatalf("pipeline %s download pct = %d, want 0", rt.Pipeline.Name, got)
		}
	}
}
