package committed

import (
	"context"
	"database/sql"
	"testing"
)

func testStore(t *testing.T) Store {
	t.Helper()
	s, err := NewSQLite(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestPipelineCRUD(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	id, err := s.CreatePipeline(ctx, &Pipeline{Name: "VR", InputDir: "/dl/vr", OutputDir: "/media/vr", PathPattern: "{Year}/{Number}", ScrapeProviders: "dmm,avwiki"})
	if err != nil {
		t.Fatal(err)
	}
	if id == 0 {
		t.Fatal("expected non-zero id")
	}

	p, err := s.GetPipeline(ctx, id)
	if err != nil || p == nil {
		t.Fatalf("get: %v", err)
	}
	if p.Name != "VR" || p.InputDir != "/dl/vr" {
		t.Fatalf("unexpected: %+v", p)
	}

	list, err := s.ListPipelines(ctx)
	if err != nil || len(list) != 1 {
		t.Fatalf("list: %v len=%d", err, len(list))
	}

	// Unique input_dir constraint
	_, err = s.CreatePipeline(ctx, &Pipeline{Name: "VR2", InputDir: "/dl/vr", OutputDir: "/media/vr2"})
	if err == nil {
		t.Fatal("expected unique constraint error")
	}

	if err := s.DeletePipeline(ctx, id); err != nil {
		t.Fatal(err)
	}
	list, _ = s.ListPipelines(ctx)
	if len(list) != 0 {
		t.Fatal("expected empty")
	}
}

func TestProviderConfig(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	cfg, err := s.GetProviderConfig(ctx, "dmm")
	if err != nil {
		t.Fatal(err)
	}
	if cfg != "{}" {
		t.Fatalf("expected empty config, got %q", cfg)
	}

	if err := s.SetProviderConfig(ctx, "dmm", `{"api_id":"x"}`); err != nil {
		t.Fatal(err)
	}
	cfg, _ = s.GetProviderConfig(ctx, "dmm")
	if cfg != `{"api_id":"x"}` {
		t.Fatalf("got %q", cfg)
	}

	list, err := s.ListProviderConfigs(ctx)
	if err != nil || len(list) != 1 {
		t.Fatalf("list: %v len=%d", err, len(list))
	}
}

func TestOutputsWithPipeline(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	id, _ := s.CreatePipeline(ctx, &Pipeline{Name: "VR", InputDir: "/dl/vr", OutputDir: "/media/vr"})

	if err := s.CreateOutput(ctx, &Output{PipelineID: id, Number: "ACHJ-057", LinkPath: "/out/a.mp4", LinkType: "hardlink"}); err != nil {
		t.Fatal(err)
	}
	ok, err := s.IsCommitted(ctx, "ACHJ-057")
	if err != nil || !ok {
		t.Fatalf("is committed: %v %v", ok, err)
	}

	outs, total, err := s.ListOutputsByPipeline(ctx, id, 10, 0, "added", "desc")
	if err != nil || total != 1 || len(outs) != 1 {
		t.Fatalf("outputs: %v total=%d len=%d", err, total, len(outs))
	}
	if outs[0].PipelineID != id {
		t.Fatalf("pipeline_id: got %d want %d", outs[0].PipelineID, id)
	}
}

func TestGetOutputByID(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	id, _ := s.CreatePipeline(ctx, &Pipeline{Name: "VR", InputDir: "/dl/vr", OutputDir: "/media/vr"})
	if err := s.CreateOutput(ctx, &Output{PipelineID: id, Number: "ACHJ-057", LinkPath: "/out/a.mp4", LinkType: "hardlink"}); err != nil {
		t.Fatal(err)
	}

	outs, _, err := s.ListOutputsByPipeline(ctx, id, 1, 0, "added", "desc")
	if err != nil || len(outs) != 1 {
		t.Fatalf("list outputs: err=%v len=%d", err, len(outs))
	}

	got, err := s.GetOutputByID(ctx, outs[0].ID)
	if err != nil {
		t.Fatalf("get output by id: %v", err)
	}
	if got.Number != "ACHJ-057" || got.LinkPath != "/out/a.mp4" {
		t.Fatalf("unexpected output: %+v", got)
	}

	if _, err := s.GetOutputByID(ctx, 999999); err == nil {
		t.Fatal("expected sql.ErrNoRows")
	} else if err != sql.ErrNoRows {
		t.Fatalf("unexpected err: %v", err)
	}
}

func TestOutputGroupsByPipeline(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	id, _ := s.CreatePipeline(ctx, &Pipeline{Name: "VR", InputDir: "/dl/vr", OutputDir: "/media/vr"})
	_ = s.CreateOutput(ctx, &Output{PipelineID: id, Number: "SIVR-296", LinkPath: "/out/SIVR-296-cd1.mp4", LinkType: "hardlink"})
	_ = s.CreateOutput(ctx, &Output{PipelineID: id, Number: "SIVR-296", LinkPath: "/out/SIVR-296-cd2.mp4", LinkType: "hardlink"})
	_ = s.CreateOutput(ctx, &Output{PipelineID: id, Number: "ACHJ-057", LinkPath: "/out/ACHJ-057.mp4", LinkType: "hardlink"})

	groups, total, err := s.ListOutputGroupsByPipeline(ctx, id, 10, 0, "number", "asc")
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 {
		t.Fatalf("total = %d, want 2", total)
	}
	if len(groups) != 2 {
		t.Fatalf("len(groups) = %d, want 2", len(groups))
	}
	if groups[0].Number != "ACHJ-057" || len(groups[0].Outputs) != 1 {
		t.Fatalf("group[0] = %+v", groups[0])
	}
	if groups[1].Number != "SIVR-296" || len(groups[1].Outputs) != 2 {
		t.Fatalf("group[1] = %+v", groups[1])
	}
}

func TestSettingsAndMetadata(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	if err := s.SetSetting(ctx, "input_dir", "/input"); err != nil {
		t.Fatal(err)
	}
	v, err := s.GetSetting(ctx, "input_dir")
	if err != nil || v != "/input" {
		t.Fatalf("setting: %q err=%v", v, err)
	}
	if err := s.UpsertMetadata(ctx, &Metadata{Number: "ACHJ-057", Title: "T"}); err != nil {
		t.Fatal(err)
	}
	m, err := s.GetMetadata(ctx, "ACHJ-057")
	if err != nil || m == nil || m.Title != "T" {
		t.Fatalf("metadata: %+v err=%v", m, err)
	}
}

func TestMigrateSettingsToPipeline(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	// Simulate legacy settings
	_ = s.SetSetting(ctx, "input_dir", "/old/input")
	_ = s.SetSetting(ctx, "output_dir", "/old/output")
	_ = s.SetSetting(ctx, "link_path_pattern", "{Year}/{Number}")
	_ = s.SetSetting(ctx, "dmm_api_id", "myid")
	_ = s.SetSetting(ctx, "dmm_affiliate_id", "myaff")
	_ = s.SetSetting(ctx, "aria2_rpc_url", "http://localhost:6800/jsonrpc")

	// Re-open to trigger migration
	// For in-memory DB we can't re-open, so test the migration function directly
	store := s.(*SQLiteStore)
	// Delete any auto-created pipelines first
	store.db.Exec("DELETE FROM pipelines")
	if err := migrateSettingsToPipeline(store.db); err != nil {
		t.Fatal(err)
	}

	list, _ := s.ListPipelines(ctx)
	if len(list) != 1 {
		t.Fatalf("expected 1 pipeline, got %d", len(list))
	}
	if list[0].InputDir != "/old/input" || list[0].OutputDir != "/old/output" {
		t.Fatalf("unexpected pipeline: %+v", list[0])
	}

	dmmCfg, _ := s.GetProviderConfig(ctx, "dmm")
	if dmmCfg == "{}" {
		t.Fatal("expected dmm config to be migrated")
	}
	aria2Cfg, _ := s.GetProviderConfig(ctx, "aria2")
	if aria2Cfg == "{}" {
		t.Fatal("expected aria2 config to be migrated")
	}
}
