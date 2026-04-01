package committed

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

func TestNewSQLiteWithLegacySchema(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "legacy.db")

	raw, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	_, err = raw.Exec(`
	CREATE TABLE IF NOT EXISTS groups (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		number TEXT UNIQUE NOT NULL
	);
	CREATE TABLE IF NOT EXISTS source_files (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		path TEXT UNIQUE NOT NULL
	);
	CREATE TABLE IF NOT EXISTS parsed_info (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		file_id INTEGER UNIQUE NOT NULL REFERENCES source_files(id),
		number TEXT NOT NULL
	);
	CREATE TABLE IF NOT EXISTS outputs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		group_id INTEGER NOT NULL REFERENCES groups(id),
		link_path TEXT NOT NULL,
		link_type TEXT NOT NULL DEFAULT 'hardlink',
		alive BOOLEAN NOT NULL DEFAULT TRUE,
		checked_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE IF NOT EXISTS merged_parts (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		group_id INTEGER NOT NULL REFERENCES groups(id),
		filename TEXT NOT NULL,
		size INTEGER NOT NULL DEFAULT 0,
		part INTEGER NOT NULL DEFAULT 0
	);
	`)
	if err != nil {
		raw.Close()
		t.Fatal(err)
	}
	_ = raw.Close()

	s, err := NewSQLite(dbPath)
	if err != nil {
		t.Fatalf("NewSQLite failed on legacy schema: %v", err)
	}
	defer s.Close()

	// After migration, legacy tables should be gone and new tables should exist
	exists, err := tableExists(s.db, "bangous")
	if err != nil {
		t.Fatal(err)
	}
	if !exists {
		t.Fatal("expected bangous table after migration")
	}

	exists, err = tableExists(s.db, "bangou_files")
	if err != nil {
		t.Fatal(err)
	}
	if !exists {
		t.Fatal("expected bangou_files table after migration")
	}

	// Old outputs table should be dropped
	exists, err = tableExists(s.db, "outputs")
	if err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Fatal("expected outputs table to be dropped after migration")
	}
}

// TestMigrationWithData verifies that data from the old outputs+metadata schema
// is correctly migrated into bangous+bangou_files+new metadata.
func TestMigrationWithData(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "migrate.db")

	// Set up a DB with the old schema (v1+v2+columns) and insert test data
	raw, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}

	stmts := []string{
		// v1
		`CREATE TABLE settings (key TEXT PRIMARY KEY, value TEXT NOT NULL)`,
		`CREATE TABLE outputs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			number TEXT NOT NULL,
			link_path TEXT NOT NULL,
			link_type TEXT NOT NULL DEFAULT 'hardlink',
			alive BOOLEAN NOT NULL DEFAULT TRUE,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			checked_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE merged_parts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			number TEXT NOT NULL,
			filename TEXT NOT NULL,
			size INTEGER NOT NULL DEFAULT 0,
			part INTEGER NOT NULL DEFAULT 0,
			merged_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE metadata (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			number TEXT UNIQUE NOT NULL,
			title TEXT NOT NULL DEFAULT '',
			plot TEXT NOT NULL DEFAULT '',
			director TEXT NOT NULL DEFAULT '',
			maker TEXT NOT NULL DEFAULT '',
			label TEXT NOT NULL DEFAULT '',
			series TEXT NOT NULL DEFAULT '',
			actors TEXT NOT NULL DEFAULT '',
			genres TEXT NOT NULL DEFAULT '',
			cover_url TEXT NOT NULL DEFAULT '',
			premiered TEXT NOT NULL DEFAULT '',
			year TEXT NOT NULL DEFAULT '',
			runtime TEXT NOT NULL DEFAULT '',
			provider TEXT NOT NULL DEFAULT '',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		// v2 columns
		`ALTER TABLE outputs ADD COLUMN src_path TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE outputs ADD COLUMN pipeline_id INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE outputs ADD COLUMN file_size INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE outputs ADD COLUMN resolution TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE outputs ADD COLUMN video_codec TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE outputs ADD COLUMN audio_codec TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE outputs ADD COLUMN duration TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE outputs ADD COLUMN bitrate TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE metadata ADD COLUMN sample_images TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE metadata ADD COLUMN rating TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE metadata ADD COLUMN review_count INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE metadata ADD COLUMN page_url TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE metadata ADD COLUMN content_id TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE metadata ADD COLUMN pipeline_id INTEGER NOT NULL DEFAULT 0`,
		// v2 tables
		`CREATE TABLE pipelines (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			input_dir TEXT NOT NULL UNIQUE,
			output_dir TEXT NOT NULL,
			path_pattern TEXT NOT NULL DEFAULT '{Number}',
			archive_dir TEXT NOT NULL DEFAULT '',
			enable_merge BOOLEAN NOT NULL DEFAULT FALSE,
			download_provider TEXT NOT NULL DEFAULT 'none',
			scrape_providers TEXT NOT NULL DEFAULT 'avwiki,dmm',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE provider_configs (provider TEXT PRIMARY KEY, config TEXT NOT NULL DEFAULT '{}')`,
		// Insert pipeline
		`INSERT INTO pipelines (name, input_dir, output_dir) VALUES ('VR', '/dl/vr', '/media/vr')`,
		// Insert outputs (2 parts for SIVR-296, 1 for ACHJ-057)
		`INSERT INTO outputs (pipeline_id, number, src_path, link_path, link_type) VALUES (1, 'SIVR-296', '/dl/vr/sivr296-1.mp4', '/media/vr/SIVR-296/SIVR-296-cd1.mp4', 'hardlink')`,
		`INSERT INTO outputs (pipeline_id, number, src_path, link_path, link_type) VALUES (1, 'SIVR-296', '/dl/vr/sivr296-2.mp4', '/media/vr/SIVR-296/SIVR-296-cd2.mp4', 'hardlink')`,
		`INSERT INTO outputs (pipeline_id, number, src_path, link_path, link_type) VALUES (1, 'ACHJ-057', '/dl/vr/achj057.mp4', '/media/vr/ACHJ-057/ACHJ-057.mp4', 'hardlink')`,
		// Insert metadata
		`INSERT INTO metadata (pipeline_id, number, title, maker, year) VALUES (1, 'SIVR-296', 'Test Title', 'SOD', '2025')`,
		`INSERT INTO metadata (pipeline_id, number, title) VALUES (1, 'ACHJ-057', 'Another')`,
	}
	for _, s := range stmts {
		if _, err := raw.Exec(s); err != nil {
			raw.Close()
			t.Fatalf("setup %s: %v", s[:40], err)
		}
	}
	raw.Close()

	// Open with NewSQLite which triggers migration
	store, err := NewSQLite(dbPath)
	if err != nil {
		t.Fatalf("NewSQLite: %v", err)
	}
	defer store.Close()
	ctx := context.Background()

	// Verify bangous were created
	bangous, total, err := store.ListBangousByPipeline(ctx, 1, 10, 0, "number", "asc")
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 {
		t.Fatalf("expected 2 bangous, got %d", total)
	}
	if bangous[0].Number != "ACHJ-057" || bangous[1].Number != "SIVR-296" {
		t.Fatalf("unexpected bangous: %+v", bangous)
	}
	if bangous[0].OutDir != "/media/vr/ACHJ-057" {
		t.Fatalf("unexpected out_dir: %q", bangous[0].OutDir)
	}

	// Verify bangou_files
	files, err := store.ListBangouFilesByBangou(ctx, bangous[1].ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 files for SIVR-296, got %d", len(files))
	}

	// Verify metadata migrated
	meta, err := store.GetMetadataByBangou(ctx, bangous[1].ID)
	if err != nil {
		t.Fatal(err)
	}
	if meta == nil || meta.Title != "Test Title" || meta.Year != "2025" {
		t.Fatalf("unexpected metadata: %+v", meta)
	}

	// Old tables should be gone
	exists, _ := tableExists(store.db, "outputs")
	if exists {
		t.Fatal("outputs table should be dropped")
	}
}
