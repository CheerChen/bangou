package committed

import (
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

	ok, err := hasColumn(s.db, "outputs", "number")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected outputs.number after migration")
	}
}
