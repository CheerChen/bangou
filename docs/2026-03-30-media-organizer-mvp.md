# Bangou MVP Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a file-watching media organizer that identifies Japanese AV serial numbers from filenames, persists them to a database, and provides a web UI for human-driven decisions (link to media library / merge parts / ignore).

**Architecture:** Single Go binary with embedded SQLite and web UI. Watches an input directory for new video files, parses filenames to extract serial numbers and part info (no network calls), persists to SQLite via repository pattern. Web UI (Go templates + htmx) lets users decide per-group actions. Executor creates hardlinks/symlinks in output directory. Merge uses mkvmerge CLI. Runs on Raspberry Pi (arm64).

**Tech Stack:** Go 1.22+, SQLite (modernc.org/sqlite, pure Go), fsnotify, mkvmerge (system), html/template + htmx for UI

**Key Design Decisions (from discussion):**
- Input directory is the universal interface — works for aria2, manual copy, rsync, anything
- Files never move from input dir; media library is built with links
- Merge outputs mkv; source parts deleted after merge; DB retains records
- No split functionality; re-merge requires re-download (cost is low, ~3-6GB per part)
- Scraping/NFO generation is out of scope for MVP (future: MetaTube API integration)

---

## File Structure

```
bangou/              # New Go module (separate from dmm-scraper)
├── main.go                   # Entry point, CLI flags, wires dependencies
├── go.mod
├── go.sum
│
├── parser/
│   ├── parser.go             # Filename → ParsedFile (number, part, tags)
│   └── parser_test.go
│
├── store/
│   ├── models.go             # SourceFile, ParsedInfo, Group, Output structs
│   ├── store.go              # Store interface definition
│   ├── sqlite.go             # SQLite implementation of Store
│   ├── sqlite_test.go
│   └── migrations.go         # Schema creation/migration
│
├── scanner/
│   ├── scanner.go            # Directory watcher (fsnotify + periodic scan)
│   └── scanner_test.go
│
├── executor/
│   ├── linker.go             # Create hardlink/symlink in output dir
│   ├── merger.go             # mkvmerge wrapper
│   ├── executor.go           # Orchestrates link/merge/cleanup per group
│   └── executor_test.go
│
├── web/
│   ├── server.go             # HTTP server setup, routes
│   ├── handlers.go           # Handler functions (list, action, manual tag)
│   ├── templates/
│   │   ├── layout.html       # Base layout
│   │   ├── index.html        # Main dashboard (pending / done / orphaned)
│   │   └── group.html        # Single group detail view
│   └── static/
│       └── htmx.min.js       # htmx (vendored, ~14KB)
│
├── checker/
│   ├── checker.go            # Periodic link health check
│   └── checker_test.go
│
└── config/
    └── config.go             # CLI flags + config struct
```

---

## Task 1: Project Bootstrap + Config

**Files:**
- Create: `bangou/go.mod`
- Create: `bangou/main.go`
- Create: `bangou/config/config.go`

- [ ] **Step 1: Initialize Go module**

```bash
mkdir -p bangou && cd bangou
go mod init github.com/zeroAlcBeer/bangou
```

- [ ] **Step 2: Create config struct with CLI flags**

```go
// config/config.go
package config

import "flag"

// Config holds CLI-only flags (not user-configurable at runtime).
// User-configurable settings (input/output dirs, aria2) live in the DB settings table.
type Config struct {
	DBPath     string
	ListenAddr string
}

func Parse() *Config {
	c := &Config{}
	flag.StringVar(&c.DBPath, "db", "bangou.db", "SQLite database path")
	flag.StringVar(&c.ListenAddr, "listen", ":8080", "web UI listen address")
	flag.Parse()
	return c
}
```

- [ ] **Step 3: Create minimal main.go**

```go
// main.go
package main

import (
	"log"
	"github.com/zeroAlcBeer/bangou/config"
)

func main() {
	cfg := config.Parse()
	log.Printf("db=%s listen=%s", cfg.DBPath, cfg.ListenAddr)
}
```

- [ ] **Step 4: Verify it builds**

```bash
cd bangou && go build -o bangou .
./bangou --input /tmp/test-input --output /tmp/test-output
```

Expected: prints config values, exits.

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "feat: bootstrap bangou project with config"
```

---

## Task 2: Filename Parser

This is the core of the MVP. Extracts serial number, part number, and tags from filenames like `twojav.com@sivr00476_3_8k.mp4`.

**Files:**
- Create: `bangou/parser/parser.go`
- Create: `bangou/parser/parser_test.go`

- [ ] **Step 1: Write failing tests covering all known filename patterns**

```go
// parser/parser_test.go
package parser

import "testing"

func TestParse(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		want     ParsedFile
	}{
		{
			name:     "standard with site prefix and part",
			filename: "twojav.com@sivr00476_3_8k.mp4",
			want:     ParsedFile{Number: "SIVR-476", Part: 3, Tags: []string{"8k"}, Ext: ".mp4"},
		},
		{
			name:     "standard hyphenated",
			filename: "ACHJ-057.mp4",
			want:     ParsedFile{Number: "ACHJ-057", Part: 0, Tags: nil, Ext: ".mp4"},
		},
		{
			name:     "no hyphen with leading zeros",
			filename: "hmn690.mp4",
			want:     ParsedFile{Number: "HMN-690", Part: 0, Tags: nil, Ext: ".mp4"},
		},
		{
			name:     "mgstage format",
			filename: "300MAAN-783.mp4",
			want:     ParsedFile{Number: "300MAAN-783", Part: 0, Tags: nil, Ext: ".mp4"},
		},
		{
			name:     "heyzo format",
			filename: "HEYZO-3421.mp4",
			want:     ParsedFile{Number: "HEYZO-3421", Part: 0, Tags: nil, Ext: ".mp4"},
		},
		{
			name:     "VR with multiple parts",
			filename: "sivr00476_1_8k.mp4",
			want:     ParsedFile{Number: "SIVR-476", Part: 1, Tags: []string{"8k"}, Ext: ".mp4"},
		},
		{
			name:     "site prefix stripped",
			filename: "xxx.com@abc-123.mp4",
			want:     ParsedFile{Number: "ABC-123", Part: 0, Tags: nil, Ext: ".mp4"},
		},
		{
			name:     "unrecognizable",
			filename: "random_video_2025.mp4",
			want:     ParsedFile{Ext: ".mp4"},
		},
		{
			name:     "mgstage format no hyphen",
			filename: "300MAAN783.mp4",
			want:     ParsedFile{Number: "300MAAN-783", Part: 0, Tags: nil, Ext: ".mp4"},
		},
		{
			name:     "part indicator with underscore",
			filename: "sivr00476_2_8k.mp4",
			want:     ParsedFile{Number: "SIVR-476", Part: 2, Tags: []string{"8k"}, Ext: ".mp4"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Parse(tt.filename)
			if got.Number != tt.want.Number {
				t.Errorf("Number = %q, want %q", got.Number, tt.want.Number)
			}
			if got.Part != tt.want.Part {
				t.Errorf("Part = %d, want %d", got.Part, tt.want.Part)
			}
			if got.Ext != tt.want.Ext {
				t.Errorf("Ext = %q, want %q", got.Ext, tt.want.Ext)
			}
		})
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd bangou && go test ./parser/ -v
```

Expected: FAIL — `Parse` not defined.

- [ ] **Step 3: Implement parser**

```go
// parser/parser.go
package parser

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

type ParsedFile struct {
	Number string   // Normalized serial: "SIVR-476"
	Part   int      // 0 = no part info, 1+ = part number
	Tags   []string // e.g. ["8k"]
	Ext    string   // e.g. ".mp4"
}

var (
	// Strip site prefix: "twojav.com@" or "xxx.com@"
	sitePrefix = regexp.MustCompile(`^[a-zA-Z0-9.-]+@`)

	// Part indicator: _1_8k, _2_8k, _3_ etc
	partRe = regexp.MustCompile(`_(\d+)_`)

	// Tag extraction: 8k, 4k, vr (after part extraction)
	tagRe = regexp.MustCompile(`(?i)\b(8k|4k|vr)\b`)

	// MGStage format: 300MAAN-783 or 300MAAN783 (digits before letters)
	mgstageRe = regexp.MustCompile(`(?i)(\d{3,4}[a-zA-Z]{2,6})-?(\d{3,4})`)

	// Heyzo format: HEYZO-3421
	heyzoRe = regexp.MustCompile(`(?i)(heyzo-\d{4})`)

	// Standard format: ABC-123 or ABC123 (letters then digits)
	standardRe = regexp.MustCompile(`(?i)([a-zA-Z]{2,5})-?(\d{3,6})`)
)

func Parse(filename string) ParsedFile {
	ext := filepath.Ext(filename)
	name := strings.TrimSuffix(filename, ext)

	// Strip site prefix
	name = sitePrefix.ReplaceAllString(name, "")

	result := ParsedFile{Ext: ext}

	// Extract part number
	if m := partRe.FindStringSubmatch(name); len(m) > 1 {
		result.Part, _ = strconv.Atoi(m[1])
		// Remove part indicator for cleaner number extraction
		name = partRe.ReplaceAllString(name, "_")
	}

	// Extract tags
	if m := tagRe.FindAllString(name, -1); len(m) > 0 {
		for _, tag := range m {
			result.Tags = append(result.Tags, strings.ToLower(tag))
		}
	}

	// Extract serial number (try patterns in order of specificity)
	result.Number = extractNumber(name)

	return result
}

func extractNumber(name string) string {
	// Heyzo: HEYZO-3421
	if m := heyzoRe.FindStringSubmatch(name); len(m) > 1 {
		return strings.ToUpper(m[1])
	}

	// MGStage: 300MAAN-783 or 300MAAN783
	if m := mgstageRe.FindStringSubmatch(name); len(m) > 2 {
		return strings.ToUpper(m[1]) + "-" + m[2]
	}

	// Standard: ABC-123 or abc123
	if m := standardRe.FindStringSubmatch(name); len(m) > 2 {
		label := strings.ToUpper(m[1])
		num := trimLeadingZeros(m[2])
		return label + "-" + num
	}

	return ""
}

// trimLeadingZeros: "00476" → "476", but keep at least 3 digits: "057" → "057"
func trimLeadingZeros(s string) string {
	n, err := strconv.Atoi(s)
	if err != nil {
		return s
	}
	result := strconv.Itoa(n)
	// Preserve minimum 3-digit formatting
	for len(result) < 3 {
		result = "0" + result
	}
	return result
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd bangou && go test ./parser/ -v
```

Expected: all PASS.

- [ ] **Step 5: Add edge case tests and iterate**

Add tests for: filenames with `-C` suffix (Chinese sub), `-cd1` multi-disc, FC2 format, etc. Fix parser as needed. These patterns can be added incrementally based on real filenames from the user's library.

- [ ] **Step 6: Commit**

```bash
git add parser/
git commit -m "feat: filename parser extracts serial number, part, tags"
```

---

## Task 3: SQLite Store + Models

**Files:**
- Create: `bangou/store/models.go`
- Create: `bangou/store/store.go`
- Create: `bangou/store/migrations.go`
- Create: `bangou/store/sqlite.go`
- Create: `bangou/store/sqlite_test.go`

- [ ] **Step 1: Define models and store interface**

```go
// store/models.go
package store

import "time"

type SourceFile struct {
	ID        int64
	Path      string // full path in input dir
	Filename  string
	Size      int64
	Ready     bool // .aria2 gone + size stable
	CreatedAt time.Time
}

type ParsedInfo struct {
	ID         int64
	FileID     int64
	Number     string  // SIVR-476
	Part       int     // 0=no part, 1+=part number
	SourceSite string  // from filename prefix
	Tags       string  // comma-separated: "8k,vr"
}

type Group struct {
	ID        int64
	Number    string // unique serial number
	Status    string // pending, done, ignored
	Action    string // link, merge_then_link, ignore
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Output struct {
	ID        int64
	GroupID   int64
	LinkPath  string // path in output dir
	LinkType  string // hardlink, symlink
	Alive     bool
	CheckedAt time.Time
}

// MergedPart records a source part that was consumed by a merge and deleted.
type MergedPart struct {
	ID       int64
	GroupID  int64
	Filename string
	Size     int64
	Part     int
}
```

```go
// store/store.go
package store

import "context"

type Store interface {
	// Files
	UpsertFile(ctx context.Context, f *SourceFile) error
	SetFileReady(ctx context.Context, path string) error
	GetFileByPath(ctx context.Context, path string) (*SourceFile, error)
	ListUnreadyFiles(ctx context.Context) ([]SourceFile, error)
	ListUnknownFiles(ctx context.Context) ([]SourceFile, error) // ready but no parsed_info

	// Parsed info
	UpsertParsed(ctx context.Context, p *ParsedInfo) error
	GetParsedByFileID(ctx context.Context, fileID int64) (*ParsedInfo, error)

	// Groups
	EnsureGroup(ctx context.Context, number string) (*Group, error)
	GetGroup(ctx context.Context, id int64) (*Group, error)
	ListGroups(ctx context.Context, status string) ([]Group, error)
	UpdateGroupAction(ctx context.Context, id int64, action string) error
	UpdateGroupStatus(ctx context.Context, id int64, status string) error
	GetGroupFiles(ctx context.Context, groupID int64) ([]SourceFile, error)

	// Outputs
	CreateOutput(ctx context.Context, o *Output) error
	ListOutputs(ctx context.Context, groupID int64) ([]Output, error)
	SetOutputAlive(ctx context.Context, id int64, alive bool) error
	ListOrphanedOutputs(ctx context.Context) ([]Output, error)

	// Merged parts
	RecordMergedPart(ctx context.Context, m *MergedPart) error
	GetMergedParts(ctx context.Context, groupID int64) ([]MergedPart, error)

	// Settings
	GetSetting(ctx context.Context, key string) (string, error)
	SetSetting(ctx context.Context, key, value string) error
	GetAllSettings(ctx context.Context) (map[string]string, error)

	// Lifecycle
	Close() error
}
```

- [ ] **Step 2: Write migration**

```go
// store/migrations.go
package store

const schemaV1 = `
CREATE TABLE IF NOT EXISTS source_files (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    path       TEXT UNIQUE NOT NULL,
    filename   TEXT NOT NULL,
    size       INTEGER NOT NULL DEFAULT 0,
    ready      BOOLEAN NOT NULL DEFAULT FALSE,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS parsed_info (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    file_id     INTEGER UNIQUE NOT NULL REFERENCES source_files(id),
    number      TEXT NOT NULL,
    part        INTEGER NOT NULL DEFAULT 0,
    source_site TEXT NOT NULL DEFAULT '',
    tags        TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_parsed_number ON parsed_info(number);

CREATE TABLE IF NOT EXISTS groups (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    number     TEXT UNIQUE NOT NULL,
    status     TEXT NOT NULL DEFAULT 'pending',
    action     TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS outputs (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    group_id   INTEGER NOT NULL REFERENCES groups(id),
    link_path  TEXT NOT NULL,
    link_type  TEXT NOT NULL DEFAULT 'hardlink',
    alive      BOOLEAN NOT NULL DEFAULT TRUE,
    checked_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS merged_parts (
    id       INTEGER PRIMARY KEY AUTOINCREMENT,
    group_id INTEGER NOT NULL REFERENCES groups(id),
    filename TEXT NOT NULL,
    size     INTEGER NOT NULL DEFAULT 0,
    part     INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS settings (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
`
```

- [ ] **Step 3: Write failing tests for core store operations**

```go
// store/sqlite_test.go
package store

import (
	"context"
	"testing"
)

func testStore(t *testing.T) Store {
	t.Helper()
	s, err := NewSQLite(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestUpsertAndGetFile(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	f := &SourceFile{Path: "/input/test.mp4", Filename: "test.mp4", Size: 1024}
	if err := s.UpsertFile(ctx, f); err != nil {
		t.Fatal(err)
	}

	got, err := s.GetFileByPath(ctx, "/input/test.mp4")
	if err != nil {
		t.Fatal(err)
	}
	if got.Filename != "test.mp4" || got.Size != 1024 || got.Ready != false {
		t.Errorf("unexpected file: %+v", got)
	}
}

func TestEnsureGroupIdempotent(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	g1, err := s.EnsureGroup(ctx, "SIVR-476")
	if err != nil {
		t.Fatal(err)
	}
	g2, err := s.EnsureGroup(ctx, "SIVR-476")
	if err != nil {
		t.Fatal(err)
	}
	if g1.ID != g2.ID {
		t.Errorf("EnsureGroup not idempotent: %d != %d", g1.ID, g2.ID)
	}
}

func TestGroupStatusTransition(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	g, _ := s.EnsureGroup(ctx, "ACHJ-057")
	if g.Status != "pending" {
		t.Errorf("initial status = %q, want pending", g.Status)
	}

	s.UpdateGroupStatus(ctx, g.ID, "done")
	g, _ = s.GetGroup(ctx, g.ID)
	if g.Status != "done" {
		t.Errorf("status = %q, want done", g.Status)
	}
}
```

- [ ] **Step 4: Run tests to verify they fail**

```bash
cd bangou && go test ./store/ -v
```

Expected: FAIL — `NewSQLite` not defined.

- [ ] **Step 5: Implement SQLite store**

```go
// store/sqlite.go
package store

import (
	"context"
	"database/sql"
	"time"

	_ "modernc.org/sqlite"
)

type SQLiteStore struct {
	db *sql.DB
}

func NewSQLite(dsn string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.Exec("PRAGMA journal_mode=WAL")
	db.Exec("PRAGMA foreign_keys=ON")

	if _, err := db.Exec(schemaV1); err != nil {
		return nil, err
	}
	return &SQLiteStore{db: db}, nil
}

func (s *SQLiteStore) Close() error { return s.db.Close() }

func (s *SQLiteStore) UpsertFile(ctx context.Context, f *SourceFile) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO source_files (path, filename, size, ready) VALUES (?, ?, ?, ?)
		 ON CONFLICT(path) DO UPDATE SET size=excluded.size, ready=excluded.ready`,
		f.Path, f.Filename, f.Size, f.Ready)
	return err
}

func (s *SQLiteStore) GetFileByPath(ctx context.Context, path string) (*SourceFile, error) {
	f := &SourceFile{}
	err := s.db.QueryRowContext(ctx,
		`SELECT id, path, filename, size, ready, created_at FROM source_files WHERE path = ?`, path).
		Scan(&f.ID, &f.Path, &f.Filename, &f.Size, &f.Ready, &f.CreatedAt)
	return f, err
}

func (s *SQLiteStore) SetFileReady(ctx context.Context, path string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE source_files SET ready = TRUE WHERE path = ?`, path)
	return err
}

func (s *SQLiteStore) ListUnreadyFiles(ctx context.Context) ([]SourceFile, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, path, filename, size, ready, created_at FROM source_files WHERE ready = FALSE`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var files []SourceFile
	for rows.Next() {
		var f SourceFile
		rows.Scan(&f.ID, &f.Path, &f.Filename, &f.Size, &f.Ready, &f.CreatedAt)
		files = append(files, f)
	}
	return files, nil
}

func (s *SQLiteStore) ListUnknownFiles(ctx context.Context) ([]SourceFile, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT sf.id, sf.path, sf.filename, sf.size, sf.ready, sf.created_at
		 FROM source_files sf
		 LEFT JOIN parsed_info pi ON pi.file_id = sf.id
		 WHERE sf.ready = TRUE AND pi.id IS NULL`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var files []SourceFile
	for rows.Next() {
		var f SourceFile
		rows.Scan(&f.ID, &f.Path, &f.Filename, &f.Size, &f.Ready, &f.CreatedAt)
		files = append(files, f)
	}
	return files, nil
}

func (s *SQLiteStore) UpsertParsed(ctx context.Context, p *ParsedInfo) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO parsed_info (file_id, number, part, source_site, tags) VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT DO NOTHING`,
		p.FileID, p.Number, p.Part, p.SourceSite, p.Tags)
	return err
}

func (s *SQLiteStore) GetParsedByFileID(ctx context.Context, fileID int64) (*ParsedInfo, error) {
	p := &ParsedInfo{}
	err := s.db.QueryRowContext(ctx,
		`SELECT id, file_id, number, part, source_site, tags FROM parsed_info WHERE file_id = ?`, fileID).
		Scan(&p.ID, &p.FileID, &p.Number, &p.Part, &p.SourceSite, &p.Tags)
	return p, err
}

func (s *SQLiteStore) EnsureGroup(ctx context.Context, number string) (*Group, error) {
	now := time.Now()
	s.db.ExecContext(ctx,
		`INSERT INTO groups (number, created_at, updated_at) VALUES (?, ?, ?)
		 ON CONFLICT(number) DO UPDATE SET updated_at = ?`,
		number, now, now, now)

	g := &Group{}
	err := s.db.QueryRowContext(ctx,
		`SELECT id, number, status, action, created_at, updated_at FROM groups WHERE number = ?`, number).
		Scan(&g.ID, &g.Number, &g.Status, &g.Action, &g.CreatedAt, &g.UpdatedAt)
	return g, err
}

func (s *SQLiteStore) GetGroup(ctx context.Context, id int64) (*Group, error) {
	g := &Group{}
	err := s.db.QueryRowContext(ctx,
		`SELECT id, number, status, action, created_at, updated_at FROM groups WHERE id = ?`, id).
		Scan(&g.ID, &g.Number, &g.Status, &g.Action, &g.CreatedAt, &g.UpdatedAt)
	return g, err
}

func (s *SQLiteStore) ListGroups(ctx context.Context, status string) ([]Group, error) {
	var rows *sql.Rows
	var err error
	if status == "" {
		rows, err = s.db.QueryContext(ctx,
			`SELECT id, number, status, action, created_at, updated_at FROM groups ORDER BY updated_at DESC`)
	} else {
		rows, err = s.db.QueryContext(ctx,
			`SELECT id, number, status, action, created_at, updated_at FROM groups WHERE status = ? ORDER BY updated_at DESC`, status)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var groups []Group
	for rows.Next() {
		var g Group
		rows.Scan(&g.ID, &g.Number, &g.Status, &g.Action, &g.CreatedAt, &g.UpdatedAt)
		groups = append(groups, g)
	}
	return groups, nil
}

func (s *SQLiteStore) UpdateGroupAction(ctx context.Context, id int64, action string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE groups SET action = ?, updated_at = ? WHERE id = ?`, action, time.Now(), id)
	return err
}

func (s *SQLiteStore) UpdateGroupStatus(ctx context.Context, id int64, status string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE groups SET status = ?, updated_at = ? WHERE id = ?`, status, time.Now(), id)
	return err
}

func (s *SQLiteStore) GetGroupFiles(ctx context.Context, groupID int64) ([]SourceFile, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT sf.id, sf.path, sf.filename, sf.size, sf.ready, sf.created_at
		 FROM source_files sf
		 JOIN parsed_info pi ON pi.file_id = sf.id
		 JOIN groups g ON g.number = pi.number
		 WHERE g.id = ?
		 ORDER BY pi.part ASC`, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var files []SourceFile
	for rows.Next() {
		var f SourceFile
		rows.Scan(&f.ID, &f.Path, &f.Filename, &f.Size, &f.Ready, &f.CreatedAt)
		files = append(files, f)
	}
	return files, nil
}

func (s *SQLiteStore) CreateOutput(ctx context.Context, o *Output) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO outputs (group_id, link_path, link_type) VALUES (?, ?, ?)`,
		o.GroupID, o.LinkPath, o.LinkType)
	return err
}

func (s *SQLiteStore) ListOutputs(ctx context.Context, groupID int64) ([]Output, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, group_id, link_path, link_type, alive, checked_at FROM outputs WHERE group_id = ?`, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var outputs []Output
	for rows.Next() {
		var o Output
		rows.Scan(&o.ID, &o.GroupID, &o.LinkPath, &o.LinkType, &o.Alive, &o.CheckedAt)
		outputs = append(outputs, o)
	}
	return outputs, nil
}

func (s *SQLiteStore) SetOutputAlive(ctx context.Context, id int64, alive bool) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE outputs SET alive = ?, checked_at = ? WHERE id = ?`, alive, time.Now(), id)
	return err
}

func (s *SQLiteStore) ListOrphanedOutputs(ctx context.Context) ([]Output, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, group_id, link_path, link_type, alive, checked_at FROM outputs WHERE alive = FALSE`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var outputs []Output
	for rows.Next() {
		var o Output
		rows.Scan(&o.ID, &o.GroupID, &o.LinkPath, &o.LinkType, &o.Alive, &o.CheckedAt)
		outputs = append(outputs, o)
	}
	return outputs, nil
}

func (s *SQLiteStore) RecordMergedPart(ctx context.Context, m *MergedPart) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO merged_parts (group_id, filename, size, part) VALUES (?, ?, ?, ?)`,
		m.GroupID, m.Filename, m.Size, m.Part)
	return err
}

func (s *SQLiteStore) GetMergedParts(ctx context.Context, groupID int64) ([]MergedPart, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, group_id, filename, size, part FROM merged_parts WHERE group_id = ? ORDER BY part ASC`, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var parts []MergedPart
	for rows.Next() {
		var m MergedPart
		rows.Scan(&m.ID, &m.GroupID, &m.Filename, &m.Size, &m.Part)
		parts = append(parts, m)
	}
	return parts, nil
}
```

- [ ] **Step 6: Run tests to verify they pass**

```bash
cd bangou && go get modernc.org/sqlite && go test ./store/ -v
```

Expected: all PASS.

- [ ] **Step 7: Commit**

```bash
git add store/
git commit -m "feat: SQLite store with models, migrations, repository interface"
```

---

## Task 4: Directory Scanner

Watches input directory for new/changed files. Combines fsnotify (real-time) with periodic scan (catch-up).

**Files:**
- Create: `bangou/scanner/scanner.go`
- Create: `bangou/scanner/scanner_test.go`

- [ ] **Step 1: Write failing test for scan logic**

```go
// scanner/scanner_test.go
package scanner

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanDir(t *testing.T) {
	dir := t.TempDir()

	// Create test files
	os.WriteFile(filepath.Join(dir, "ACHJ-057.mp4"), make([]byte, 1024), 0644)
	os.WriteFile(filepath.Join(dir, "ACHJ-057.mp4.aria2"), []byte("downloading"), 0644)
	os.WriteFile(filepath.Join(dir, "HMN-690.mp4"), make([]byte, 2048), 0644)
	os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("not a video"), 0644)

	files, err := ScanDir(dir)
	if err != nil {
		t.Fatal(err)
	}

	// HMN-690 should be ready (no .aria2)
	// ACHJ-057 should not be ready (.aria2 exists)
	// readme.txt should be ignored (not a video)
	if len(files) != 2 {
		t.Fatalf("got %d files, want 2", len(files))
	}

	byName := map[string]ScannedFile{}
	for _, f := range files {
		byName[f.Filename] = f
	}

	if byName["HMN-690.mp4"].Ready != true {
		t.Error("HMN-690 should be ready")
	}
	if byName["ACHJ-057.mp4"].Ready != false {
		t.Error("ACHJ-057 should not be ready")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd bangou && go test ./scanner/ -v
```

Expected: FAIL.

- [ ] **Step 3: Implement scanner**

```go
// scanner/scanner.go
package scanner

import (
	"os"
	"path/filepath"
	"strings"
)

var videoExts = map[string]bool{
	".mp4": true, ".mkv": true, ".avi": true, ".wmv": true,
}

type ScannedFile struct {
	Path     string
	Filename string
	Size     int64
	Ready    bool // no .aria2 companion
}

// ScanDir does a single pass over the directory, returns all video files.
func ScanDir(dir string) ([]ScannedFile, error) {
	var files []ScannedFile

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	// First pass: collect .aria2 files
	aria2Files := map[string]bool{}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".aria2") {
			// "foo.mp4.aria2" → "foo.mp4"
			aria2Files[strings.TrimSuffix(e.Name(), ".aria2")] = true
		}
	}

	// Second pass: collect video files
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if !videoExts[ext] {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		files = append(files, ScannedFile{
			Path:     filepath.Join(dir, e.Name()),
			Filename: e.Name(),
			Size:     info.Size(),
			Ready:    !aria2Files[e.Name()],
		})
	}

	return files, nil
}
```

- [ ] **Step 4: Run tests**

```bash
cd bangou && go test ./scanner/ -v
```

Expected: PASS.

- [ ] **Step 5: Add fsnotify watcher**

```go
// Add to scanner/scanner.go

import (
	"context"
	"log"
	"time"

	"github.com/fsnotify/fsnotify"
)

type EventType int

const (
	FileReady EventType = iota
	FileNew
)

type Event struct {
	Type EventType
	File ScannedFile
}

// Watch starts watching a directory via fsnotify. Sends events on the channel.
// Does an initial full scan on start, then reacts to filesystem events only.
func Watch(ctx context.Context, dir string, ch chan<- Event) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	if err := watcher.Add(dir); err != nil {
		watcher.Close()
		return err
	}

	// Initial scan to pick up existing files
	go DoScan(dir, ch)

	go func() {
		defer watcher.Close()
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-watcher.Events:
				if event.Op&(fsnotify.Create|fsnotify.Rename) != 0 {
					// Small delay to let file finish writing
					time.Sleep(2 * time.Second)
					DoScan(dir, ch)
				}
				if event.Op&fsnotify.Remove != 0 {
					// .aria2 file removed = download complete, rescan
					if strings.HasSuffix(event.Name, ".aria2") {
						time.Sleep(1 * time.Second)
						DoScan(dir, ch)
					}
				}
			case err := <-watcher.Errors:
				log.Printf("watcher error: %v", err)
			}
		}
	}()

	return nil
}

func DoScan(dir string, ch chan<- Event) {
	files, err := ScanDir(dir)
	if err != nil {
		log.Printf("scan error: %v", err)
		return
	}
	for _, f := range files {
		if f.Ready {
			ch <- Event{Type: FileReady, File: f}
		} else {
			ch <- Event{Type: FileNew, File: f}
		}
	}
}
```

- [ ] **Step 6: Commit**

```bash
go get github.com/fsnotify/fsnotify
git add scanner/
git commit -m "feat: directory scanner with fsnotify + periodic fallback"
```

---

## Task 5: Pipeline — Scanner → Parser → Store

Wire scanner events through parser into store. This is the core loop.

**Files:**
- Modify: `bangou/main.go`

- [ ] **Step 1: Write the processing pipeline**

```go
// main.go
package main

import (
	"context"
	"log"
	"os"
	"os/signal"

	"github.com/zeroAlcBeer/bangou/config"
	"github.com/zeroAlcBeer/bangou/parser"
	"github.com/zeroAlcBeer/bangou/scanner"
	"github.com/zeroAlcBeer/bangou/store"
)

func main() {
	cfg := config.Parse()

	db, err := store.NewSQLite(cfg.DBPath)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	// Read input dir from DB settings; if not set, just start web UI for configuration
	inputDir, _ := db.GetSetting(ctx, "input_dir")
	events := make(chan scanner.Event, 100)
	if inputDir != "" {
		if err := scanner.Watch(ctx, inputDir, events); err != nil {
			log.Fatalf("watch: %v", err)
		}
		log.Printf("watching %s", inputDir)
	} else {
		log.Printf("no input directory configured, open web UI to set up")
	}

	log.Printf("web UI at http://%s", cfg.ListenAddr)

	// TODO: start web server (Task 7)
	// TODO: start link checker (Task 8)

	processEvents(ctx, db, events)
}

func processEvents(ctx context.Context, db store.Store, events <-chan scanner.Event) {
	for {
		select {
		case <-ctx.Done():
			return
		case ev := <-events:
			handleEvent(ctx, db, ev)
		}
	}
}

func handleEvent(ctx context.Context, db store.Store, ev scanner.Event) {
	f := ev.File

	// Upsert source file
	sf := &store.SourceFile{
		Path:     f.Path,
		Filename: f.Filename,
		Size:     f.Size,
		Ready:    f.Ready,
	}
	if err := db.UpsertFile(ctx, sf); err != nil {
		log.Printf("upsert file %s: %v", f.Filename, err)
		return
	}

	if !f.Ready {
		return // still downloading
	}

	// Mark ready
	db.SetFileReady(ctx, f.Path)

	// Parse filename
	parsed := parser.Parse(f.Filename)
	if parsed.Number == "" {
		log.Printf("unknown format: %s", f.Filename)
		return // will show as "unknown" in UI
	}

	// Get file from DB (need the ID)
	dbFile, err := db.GetFileByPath(ctx, f.Path)
	if err != nil {
		log.Printf("get file %s: %v", f.Path, err)
		return
	}

	// Save parsed info
	pi := &store.ParsedInfo{
		FileID: dbFile.ID,
		Number: parsed.Number,
		Part:   parsed.Part,
		Tags:   joinTags(parsed.Tags),
	}
	if err := db.UpsertParsed(ctx, pi); err != nil {
		log.Printf("upsert parsed %s: %v", parsed.Number, err)
		return
	}

	// Ensure group exists
	group, err := db.EnsureGroup(ctx, parsed.Number)
	if err != nil {
		log.Printf("ensure group %s: %v", parsed.Number, err)
		return
	}

	log.Printf("indexed: %s (part=%d) → group %s [%s]", f.Filename, parsed.Part, group.Number, group.Status)
}

func joinTags(tags []string) string {
	if len(tags) == 0 {
		return ""
	}
	result := tags[0]
	for _, t := range tags[1:] {
		result += "," + t
	}
	return result
}
```

- [ ] **Step 2: Manual integration test**

```bash
mkdir -p /tmp/test-input /tmp/test-output
cp some_test_video.mp4 /tmp/test-input/twojav.com@sivr00476_1_8k.mp4

cd bangou && go run . --input /tmp/test-input --output /tmp/test-output --db /tmp/test.db
```

Expected: log output shows `indexed: twojav.com@sivr00476_1_8k.mp4 (part=1) → group SIVR-476 [pending]`

- [ ] **Step 3: Commit**

```bash
git add main.go
git commit -m "feat: wire scanner → parser → store pipeline"
```

---

## Task 6: Executor (Link + Merge)

**Files:**
- Create: `bangou/executor/linker.go`
- Create: `bangou/executor/merger.go`
- Create: `bangou/executor/executor.go`
- Create: `bangou/executor/executor_test.go`

- [ ] **Step 1: Write failing test for linker**

```go
// executor/executor_test.go
package executor

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestLinkFile(t *testing.T) {
	srcDir := t.TempDir()
	outDir := t.TempDir()

	// Create source file
	srcPath := filepath.Join(srcDir, "test.mp4")
	os.WriteFile(srcPath, []byte("video data"), 0644)

	linkPath, err := LinkFile(srcPath, outDir, "ACHJ-057", "hardlink")
	if err != nil {
		t.Fatal(err)
	}

	expected := filepath.Join(outDir, "ACHJ-057", "ACHJ-057.mp4")
	if linkPath != expected {
		t.Errorf("linkPath = %q, want %q", linkPath, expected)
	}

	// Verify link exists and has same content
	data, err := os.ReadFile(linkPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "video data" {
		t.Error("link content mismatch")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd bangou && go test ./executor/ -v
```

- [ ] **Step 3: Implement linker**

```go
// executor/linker.go
package executor

import (
	"fmt"
	"os"
	"path/filepath"
)

// LinkFile creates a link from srcPath to outputDir/number/number.ext
// Returns the full path of the created link.
func LinkFile(srcPath, outputDir, number, linkType string) (string, error) {
	ext := filepath.Ext(srcPath)
	dir := filepath.Join(outputDir, number)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("mkdir %s: %w", dir, err)
	}

	linkPath := filepath.Join(dir, number+ext)

	switch linkType {
	case "hardlink":
		if err := os.Link(srcPath, linkPath); err != nil {
			return "", fmt.Errorf("hardlink: %w", err)
		}
	case "symlink":
		if err := os.Symlink(srcPath, linkPath); err != nil {
			return "", fmt.Errorf("symlink: %w", err)
		}
	default:
		return "", fmt.Errorf("unknown link type: %s", linkType)
	}

	return linkPath, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
cd bangou && go test ./executor/ -v -run TestLinkFile
```

- [ ] **Step 5: Write failing test for merger**

```go
// Add to executor/executor_test.go

func TestMergeFiles(t *testing.T) {
	// Skip if mkvmerge not installed
	if _, err := exec.LookPath("mkvmerge"); err != nil {
		t.Skip("mkvmerge not installed")
	}

	// This test needs real video files to work with mkvmerge.
	// For unit testing, we test the command construction.
	parts := []string{"/tmp/part1.mp4", "/tmp/part2.mp4", "/tmp/part3.mp4"}
	output := "/tmp/output.mkv"

	cmd := BuildMergeCommand(parts, output)
	// mkvmerge -o output.mkv part1.mp4 + part2.mp4 + part3.mp4
	if cmd.Path == "" {
		t.Error("command not built")
	}
}
```

- [ ] **Step 6: Implement merger**

```go
// executor/merger.go
package executor

import (
	"fmt"
	"os/exec"
)

// BuildMergeCommand constructs the mkvmerge command for appending parts.
// mkvmerge -o output.mkv part1.mp4 + part2.mp4 + part3.mp4
func BuildMergeCommand(parts []string, output string) *exec.Cmd {
	args := []string{"-o", output}
	for i, p := range parts {
		if i > 0 {
			args = append(args, "+")
		}
		args = append(args, p)
	}
	return exec.Command("mkvmerge", args...)
}

// MergeFiles runs mkvmerge to concatenate parts into a single output file.
func MergeFiles(parts []string, output string) error {
	if len(parts) == 0 {
		return fmt.Errorf("no parts to merge")
	}
	if len(parts) == 1 {
		return fmt.Errorf("only one part, no merge needed")
	}

	cmd := BuildMergeCommand(parts, output)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("mkvmerge: %w\n%s", err, string(out))
	}
	return nil
}
```

- [ ] **Step 7: Implement executor orchestrator**

```go
// executor/executor.go
package executor

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/zeroAlcBeer/bangou/store"
)

type Executor struct {
	store     store.Store
	outputDir string
	linkType  string // "hardlink" or "symlink"
}

func New(s store.Store, outputDir, linkType string) *Executor {
	return &Executor{store: s, outputDir: outputDir, linkType: linkType}
}

// ExecuteGroup processes a group based on its action.
func (e *Executor) ExecuteGroup(ctx context.Context, groupID int64, action string) error {
	group, err := e.store.GetGroup(ctx, groupID)
	if err != nil {
		return err
	}

	files, err := e.store.GetGroupFiles(ctx, groupID)
	if err != nil {
		return err
	}

	if len(files) == 0 {
		return fmt.Errorf("no files in group %s", group.Number)
	}

	switch action {
	case "link":
		return e.doLink(ctx, group, files)
	case "merge_then_link":
		return e.doMergeThenLink(ctx, group, files)
	default:
		return fmt.Errorf("unknown action: %s", action)
	}
}

func (e *Executor) doLink(ctx context.Context, group *store.Group, files []store.SourceFile) error {
	// Link the first (or only) file
	src := files[0]
	linkPath, err := LinkFile(src.Path, e.outputDir, group.Number, e.linkType)
	if err != nil {
		return err
	}

	if err := e.store.CreateOutput(ctx, &store.Output{
		GroupID:  group.ID,
		LinkPath: linkPath,
		LinkType: e.linkType,
	}); err != nil {
		return err
	}

	e.store.UpdateGroupAction(ctx, group.ID, "link")
	e.store.UpdateGroupStatus(ctx, group.ID, "done")
	log.Printf("linked: %s → %s", src.Filename, linkPath)
	return nil
}

func (e *Executor) doMergeThenLink(ctx context.Context, group *store.Group, files []store.SourceFile) error {
	if len(files) < 2 {
		return fmt.Errorf("need at least 2 files to merge, got %d", len(files))
	}

	// Collect source paths
	var parts []string
	for _, f := range files {
		parts = append(parts, f.Path)
	}

	// Merge into output dir
	outDir := filepath.Join(e.outputDir, group.Number)
	os.MkdirAll(outDir, 0755)
	mergedPath := filepath.Join(outDir, group.Number+".mkv")

	e.store.UpdateGroupStatus(ctx, group.ID, "merging")

	if err := MergeFiles(parts, mergedPath); err != nil {
		e.store.UpdateGroupStatus(ctx, group.ID, "merge_error")
		return err
	}

	// Record merged parts and delete source files
	for _, f := range files {
		parsed, _ := e.store.GetParsedByFileID(ctx, f.ID)
		part := 0
		if parsed != nil {
			part = parsed.Part
		}
		e.store.RecordMergedPart(ctx, &store.MergedPart{
			GroupID:  group.ID,
			Filename: f.Filename,
			Size:     f.Size,
			Part:     part,
		})
		if err := os.Remove(f.Path); err != nil {
			log.Printf("warn: could not delete source %s: %v", f.Path, err)
		}
	}

	// Record output
	e.store.CreateOutput(ctx, &store.Output{
		GroupID:  group.ID,
		LinkPath: mergedPath,
		LinkType: "file", // merged file, not a link
	})

	e.store.UpdateGroupAction(ctx, group.ID, "merge_then_link")
	e.store.UpdateGroupStatus(ctx, group.ID, "done")
	log.Printf("merged %d parts → %s", len(files), mergedPath)
	return nil
}
```

- [ ] **Step 8: Run all executor tests**

```bash
cd bangou && go test ./executor/ -v
```

- [ ] **Step 9: Commit**

```bash
git add executor/
git commit -m "feat: executor with link and merge-then-link actions"
```

---

## Task 7: Web UI

Minimal web UI with htmx for in-place updates. No JS framework.

**Files:**
- Create: `bangou/web/server.go`
- Create: `bangou/web/handlers.go`
- Create: `bangou/web/templates/layout.html`
- Create: `bangou/web/templates/index.html`
- Create: `bangou/web/templates/group.html`
- Vendor: `bangou/web/static/htmx.min.js`

- [ ] **Step 1: Create layout template**

```html
<!-- web/templates/layout.html -->
<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>Media Organizer</title>
    <script src="/static/htmx.min.js"></script>
    <style>
        * { box-sizing: border-box; margin: 0; padding: 0; }
        body { font-family: -apple-system, system-ui, sans-serif; background: #1a1a2e; color: #e0e0e0; padding: 20px; }
        h1 { margin-bottom: 20px; color: #fff; }
        h2 { margin: 20px 0 10px; color: #aaa; font-size: 14px; text-transform: uppercase; }
        .card { background: #16213e; border-radius: 8px; padding: 16px; margin-bottom: 12px; }
        .card-header { display: flex; justify-content: space-between; align-items: center; }
        .number { font-size: 18px; font-weight: bold; color: #fff; }
        .tags { display: flex; gap: 6px; }
        .tag { background: #0f3460; padding: 2px 8px; border-radius: 4px; font-size: 12px; }
        .tag.vr { background: #e94560; }
        .parts { margin: 10px 0; color: #aaa; font-size: 14px; }
        .actions { display: flex; gap: 8px; margin-top: 12px; }
        .btn { padding: 8px 16px; border: none; border-radius: 6px; cursor: pointer; font-size: 14px; }
        .btn-primary { background: #533483; color: #fff; }
        .btn-merge { background: #e94560; color: #fff; }
        .btn-ignore { background: #333; color: #aaa; }
        .btn:hover { opacity: 0.8; }
        .status { font-size: 12px; padding: 2px 8px; border-radius: 4px; }
        .status-done { background: #1b4332; color: #95d5b2; }
        .status-orphaned { background: #7f4f24; color: #ddb892; }
        .status-merging { background: #1d3557; color: #a8dadc; }
        .manual-input { margin-top: 10px; display: flex; gap: 8px; }
        .manual-input input { padding: 6px; border-radius: 4px; border: 1px solid #333; background: #0a0a1a; color: #fff; }
        .scan-btn { float: right; }
    </style>
</head>
<body>
    <h1>Media Organizer <button class="btn btn-primary scan-btn" hx-post="/api/scan" hx-swap="none">Scan Now</button></h1>
    {{template "content" .}}
</body>
</html>
```

- [ ] **Step 2: Create index template**

```html
<!-- web/templates/index.html -->
{{define "content"}}

{{if .Unknown}}
<h2>Unknown ({{len .Unknown}})</h2>
{{range .Unknown}}
<div class="card">
    <div class="card-header">
        <span class="number">???</span>
        <span class="parts">{{.Filename}}</span>
    </div>
    <form class="manual-input" hx-post="/api/files/{{.ID}}/tag" hx-target="closest .card" hx-swap="outerHTML">
        <input type="text" name="number" placeholder="Enter serial number, e.g. SIVR-476">
        <button class="btn btn-primary" type="submit">Set</button>
        <button class="btn btn-ignore" hx-post="/api/files/{{.ID}}/ignore" hx-target="closest .card" hx-swap="outerHTML">Ignore</button>
    </form>
</div>
{{end}}
{{end}}

{{if .Pending}}
<h2>Pending ({{len .Pending}})</h2>
{{range .Pending}}
<div class="card" id="group-{{.Group.ID}}">
    <div class="card-header">
        <span class="number">{{.Group.Number}}</span>
        <div class="tags">
            {{range .Tags}}<span class="tag {{.}}">{{.}}</span>{{end}}
        </div>
    </div>
    <div class="parts">
        {{range .Files}}
            Part {{.Part}}: {{.Filename}} ({{printf "%.1f" .SizeGB}}GB) {{if .Ready}}✓{{else}}⏳{{end}}<br>
        {{end}}
    </div>
    <div class="actions">
        {{if gt (len .Files) 1}}
        <button class="btn btn-merge" hx-post="/api/groups/{{.Group.ID}}/action" hx-vals='{"action":"merge_then_link"}' hx-target="#group-{{.Group.ID}}" hx-swap="outerHTML">Merge & Add to Library</button>
        {{end}}
        <button class="btn btn-primary" hx-post="/api/groups/{{.Group.ID}}/action" hx-vals='{"action":"link"}' hx-target="#group-{{.Group.ID}}" hx-swap="outerHTML">Add to Library</button>
        <button class="btn btn-ignore" hx-post="/api/groups/{{.Group.ID}}/action" hx-vals='{"action":"ignore"}' hx-target="#group-{{.Group.ID}}" hx-swap="outerHTML">Ignore</button>
    </div>
</div>
{{end}}
{{end}}

{{if .Done}}
<h2>In Library ({{len .Done}})</h2>
{{range .Done}}
<div class="card">
    <div class="card-header">
        <span class="number">{{.Group.Number}}</span>
        <span class="status status-done">✓ linked</span>
    </div>
    {{if .HasNewParts}}<div class="parts" style="color:#e94560">⚠ New parts available</div>{{end}}
    {{if not .LinkAlive}}<div class="parts" style="color:#ddb892">⚠ Link missing (deleted from Emby?)</div>
    <div class="actions">
        <button class="btn btn-primary" hx-post="/api/groups/{{.Group.ID}}/rebuild">Rebuild Link</button>
        <button class="btn btn-ignore" hx-post="/api/groups/{{.Group.ID}}/action" hx-vals='{"action":"ignore"}'>Dismiss</button>
    </div>
    {{end}}
</div>
{{end}}
{{end}}

{{end}}
```

- [ ] **Step 2b: Create group.html template (for htmx partial updates)**

```html
<!-- web/templates/group.html -->
{{define "group-card"}}
<div class="card" id="group-{{.Group.ID}}">
    <div class="card-header">
        <span class="number">{{.Group.Number}}</span>
        {{if eq .Group.Status "done"}}<span class="status status-done">✓ linked</span>{{end}}
        {{if eq .Group.Status "ignored"}}<span class="status" style="color:#666">ignored</span>{{end}}
        {{if eq .Group.Status "merging"}}<span class="status status-merging">merging...</span>{{end}}
    </div>
    {{if eq .Group.Status "pending"}}
    <div class="parts">
        {{range .Files}}
            Part {{.Part}}: {{.Filename}} ({{printf "%.1f" .SizeGB}}GB) {{if .Ready}}✓{{else}}⏳{{end}}<br>
        {{end}}
    </div>
    <div class="actions">
        {{if gt (len .Files) 1}}
        <button class="btn btn-merge" hx-post="/api/groups/{{.Group.ID}}/action" hx-vals='{"action":"merge_then_link"}' hx-target="#group-{{.Group.ID}}" hx-swap="outerHTML">Merge & Add to Library</button>
        {{end}}
        <button class="btn btn-primary" hx-post="/api/groups/{{.Group.ID}}/action" hx-vals='{"action":"link"}' hx-target="#group-{{.Group.ID}}" hx-swap="outerHTML">Add to Library</button>
        <button class="btn btn-ignore" hx-post="/api/groups/{{.Group.ID}}/action" hx-vals='{"action":"ignore"}' hx-target="#group-{{.Group.ID}}" hx-swap="outerHTML">Ignore</button>
    </div>
    {{end}}
</div>
{{end}}
```

- [ ] **Step 3: Implement handlers**

```go
// web/handlers.go
package web

import (
	"context"
	"net/http"
	"strconv"

	"github.com/zeroAlcBeer/bangou/executor"
	"github.com/zeroAlcBeer/bangou/store"
)

type Handlers struct {
	store    store.Store
	executor *executor.Executor
	scanFn   func() // trigger a manual scan
}

type GroupView struct {
	Group       store.Group
	Files       []FileView
	Tags        []string
	HasNewParts bool
	LinkAlive   bool
}

type FileView struct {
	store.SourceFile
	Part   int
	SizeGB float64
}

func (h *Handlers) Index(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	pending, _ := h.store.ListGroups(ctx, "pending")
	done, _ := h.store.ListGroups(ctx, "done")
	unknown, _ := h.store.ListUnknownFiles(ctx)

	data := map[string]interface{}{
		"Pending": h.buildGroupViews(ctx, pending),
		"Done":    h.buildGroupViews(ctx, done),
		"Unknown": unknown,
	}

	renderTemplate(w, "layout.html", data)
}

func (h *Handlers) GroupAction(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	idStr := r.PathValue("id")
	id, _ := strconv.ParseInt(idStr, 10, 64)

	action := r.FormValue("action")

	switch action {
	case "link", "merge_then_link":
		if err := h.executor.ExecuteGroup(ctx, id, action); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
	case "ignore":
		h.store.UpdateGroupStatus(ctx, id, "ignored")
	}

	// Return updated card via htmx
	group, _ := h.store.GetGroup(ctx, id)
	gv := h.buildGroupView(ctx, *group)
	renderTemplate(w, "group-card", gv)
}

func (h *Handlers) TriggerScan(w http.ResponseWriter, r *http.Request) {
	h.scanFn()
	w.WriteHeader(204)
}

func (h *Handlers) ManualTag(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	idStr := r.PathValue("id")
	id, _ := strconv.ParseInt(idStr, 10, 64)
	number := r.FormValue("number")

	if number == "" {
		http.Error(w, "number is required", 400)
		return
	}

	h.store.UpsertParsed(ctx, &store.ParsedInfo{
		FileID: id,
		Number: number,
	})
	h.store.EnsureGroup(ctx, number)

	// Return empty div to remove the card from unknown list
	w.Write([]byte(`<div></div>`))
}

func (h *Handlers) buildGroupViews(ctx context.Context, groups []store.Group) []GroupView {
	var views []GroupView
	for _, g := range groups {
		views = append(views, h.buildGroupView(ctx, g))
	}
	return views
}

func (h *Handlers) buildGroupView(ctx context.Context, g store.Group) GroupView {
	files, _ := h.store.GetGroupFiles(ctx, g.ID)
	var fileViews []FileView
	for _, f := range files {
		p, _ := h.store.GetParsedByFileID(ctx, f.ID)
		part := 0
		if p != nil {
			part = p.Part
		}
		fileViews = append(fileViews, FileView{
			SourceFile: f,
			Part:       part,
			SizeGB:     float64(f.Size) / (1024 * 1024 * 1024),
		})
	}

	return GroupView{
		Group: g,
		Files: fileViews,
	}
}
```

- [ ] **Step 4: Implement server setup**

```go
// web/server.go
package web

import (
	"embed"
	"html/template"
	"net/http"

	"github.com/zeroAlcBeer/bangou/executor"
	"github.com/zeroAlcBeer/bangou/store"
)

//go:embed templates/*.html static/*
var content embed.FS

var templates *template.Template

func init() {
	templates = template.Must(template.ParseFS(content, "templates/*.html"))
}

func renderTemplate(w http.ResponseWriter, name string, data interface{}) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	templates.ExecuteTemplate(w, name, data)
}

func NewServer(s store.Store, exec *executor.Executor, scanFn func()) http.Handler {
	h := &Handlers{store: s, executor: exec, scanFn: scanFn}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /", h.Index)
	mux.HandleFunc("GET /settings", h.Settings)
	mux.HandleFunc("POST /api/settings", h.SaveSettings)
	mux.HandleFunc("POST /api/groups/{id}/action", h.GroupAction)
	mux.HandleFunc("POST /api/scan", h.TriggerScan)
	mux.HandleFunc("POST /api/files/{id}/tag", h.ManualTag)
	mux.Handle("GET /static/", http.FileServerFS(content))

	return mux
}
```

- [ ] **Step 5: Create settings template and handlers**

```html
<!-- web/templates/settings.html -->
{{define "content"}}
<h2>Settings</h2>
<form hx-post="/api/settings" hx-swap="outerHTML" hx-target="#settings-form">
<div id="settings-form">
    <div class="card">
        <label>Input Directory (files to process)</label>
        <input type="text" name="input_dir" value="{{.input_dir}}" style="width:100%; padding:8px; margin:8px 0; background:#0a0a1a; color:#fff; border:1px solid #333; border-radius:4px;">

        <label>Output Directory (media library)</label>
        <input type="text" name="output_dir" value="{{.output_dir}}" style="width:100%; padding:8px; margin:8px 0; background:#0a0a1a; color:#fff; border:1px solid #333; border-radius:4px;">
    </div>

    <div class="card">
        <label>aria2 RPC URL (optional)</label>
        <input type="text" name="aria2_rpc_url" value="{{.aria2_rpc_url}}" placeholder="http://localhost:6800/jsonrpc" style="width:100%; padding:8px; margin:8px 0; background:#0a0a1a; color:#fff; border:1px solid #333; border-radius:4px;">

        <label>aria2 Token (optional)</label>
        <input type="password" name="aria2_token" value="{{.aria2_token}}" style="width:100%; padding:8px; margin:8px 0; background:#0a0a1a; color:#fff; border:1px solid #333; border-radius:4px;">
    </div>

    <div class="card" style="color:#aaa">
        <strong>Auto-detected</strong><br>
        mkvmerge: {{if .mkvmerge_available}}✓ {{.mkvmerge_path}}{{else}}✗ not found (merge disabled){{end}}<br>
        Link mode: {{.link_type}}
    </div>

    <button class="btn btn-primary" type="submit" style="margin-top:12px">Save</button>
</div>
</form>
{{end}}
```

Add to `web/handlers.go`:

```go
func (h *Handlers) Settings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	settings, _ := h.store.GetAllSettings(ctx)

	// Auto-detect mkvmerge
	mkvPath, err := exec.LookPath("mkvmerge")
	settings["mkvmerge_available"] = "true"
	settings["mkvmerge_path"] = mkvPath
	if err != nil {
		settings["mkvmerge_available"] = ""
		settings["mkvmerge_path"] = ""
	}

	// Auto-detect link type
	settings["link_type"] = detectLinkType(settings["input_dir"], settings["output_dir"])

	renderTemplate(w, "layout.html", settings)
}

func (h *Handlers) SaveSettings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	r.ParseForm()

	keys := []string{"input_dir", "output_dir", "aria2_rpc_url", "aria2_token"}
	for _, k := range keys {
		if v := r.FormValue(k); v != "" {
			h.store.SetSetting(ctx, k, v)
		}
	}

	// Redirect back to settings to show updated values
	http.Redirect(w, r, "/settings", http.StatusSeeOther)
}
```

- [ ] **Step 6: Download htmx and vendor it**

```bash
curl -o bangou/web/static/htmx.min.js https://unpkg.com/htmx.org@2.0.4/dist/htmx.min.js
```

- [ ] **Step 6: Commit**

```bash
git add web/
git commit -m "feat: web UI with htmx for group actions"
```

---

## Task 8: Link Health Checker

Periodically checks if links in output dir still exist (Emby may have deleted them).

**Files:**
- Create: `bangou/checker/checker.go`
- Create: `bangou/checker/checker_test.go`

- [ ] **Step 1: Write failing test**

```go
// checker/checker_test.go
package checker

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheckLink(t *testing.T) {
	dir := t.TempDir()
	existing := filepath.Join(dir, "exists.mp4")
	os.WriteFile(existing, []byte("data"), 0644)

	if !CheckLink(existing) {
		t.Error("existing file should report alive")
	}
	if CheckLink(filepath.Join(dir, "gone.mp4")) {
		t.Error("missing file should report not alive")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd bangou && go test ./checker/ -v
```

- [ ] **Step 3: Implement checker**

```go
// checker/checker.go
package checker

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/zeroAlcBeer/bangou/store"
)

func CheckLink(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// Run periodically checks all outputs and updates alive status.
func Run(ctx context.Context, s store.Store, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			check(ctx, s)
		}
	}
}

func check(ctx context.Context, s store.Store) {
	groups, _ := s.ListGroups(ctx, "done")
	for _, g := range groups {
		outputs, _ := s.ListOutputs(ctx, g.ID)
		for _, o := range outputs {
			alive := CheckLink(o.LinkPath)
			if alive != o.Alive {
				s.SetOutputAlive(ctx, o.ID, alive)
				if !alive {
					log.Printf("orphaned: %s → %s", g.Number, o.LinkPath)
				}
			}
		}
	}
}
```

- [ ] **Step 4: Run tests**

```bash
cd bangou && go test ./checker/ -v
```

- [ ] **Step 5: Commit**

```bash
git add checker/
git commit -m "feat: periodic link health checker"
```

---

## Task 9: Wire Everything in main.go

Connect all components: scanner, pipeline, executor, web server, checker.

**Files:**
- Modify: `bangou/main.go`

- [ ] **Step 1: Update main.go with full wiring**

```go
// main.go — update to wire all components
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/zeroAlcBeer/bangou/checker"
	"github.com/zeroAlcBeer/bangou/config"
	"github.com/zeroAlcBeer/bangou/executor"
	"github.com/zeroAlcBeer/bangou/parser"
	"github.com/zeroAlcBeer/bangou/scanner"
	"github.com/zeroAlcBeer/bangou/store"
	"github.com/zeroAlcBeer/bangou/web"
)

func main() {
	cfg := config.Parse()

	db, err := store.NewSQLite(cfg.DBPath)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	// Read settings from DB
	settings, _ := db.GetAllSettings(ctx)
	inputDir := settings["input_dir"]
	outputDir := settings["output_dir"]

	// Auto-detect link type based on filesystem
	linkType := detectLinkType(inputDir, outputDir) // "hardlink" or "symlink"

	exec := executor.New(db, outputDir, linkType)

	events := make(chan scanner.Event, 100)

	scanFn := func() {
		dir, _ := db.GetSetting(ctx, "input_dir")
		if dir != "" {
			go scanner.DoScan(dir, events)
		}
	}

	// Start watcher if input dir is configured
	if inputDir != "" {
		if err := scanner.Watch(ctx, inputDir, events); err != nil {
			log.Printf("warn: watch failed: %v", err)
		} else {
			log.Printf("watching %s", inputDir)
		}
	} else {
		log.Printf("no input directory configured — open web UI to set up")
	}

	// Start link health checker (every hour)
	go checker.Run(ctx, db, 1*time.Hour)

	// Start web server
	srv := web.NewServer(db, exec, scanFn)
	go func() {
		log.Printf("web UI: http://%s", cfg.ListenAddr)
		if err := http.ListenAndServe(cfg.ListenAddr, srv); err != nil {
			log.Fatal(err)
		}
	}()

	// Process events
	processEvents(ctx, db, events)
}

// detectLinkType checks if input and output are on the same filesystem.
func detectLinkType(input, output string) string {
	if input == "" || output == "" {
		return "symlink"
	}
	var statIn, statOut syscall.Stat_t
	if syscall.Stat(input, &statIn) != nil || syscall.Stat(output, &statOut) != nil {
		return "symlink"
	}
	if statIn.Dev == statOut.Dev {
		return "hardlink"
	}
	return "symlink"
}

// processEvents and handleEvent remain the same as Task 5
```

- [ ] **Step 2: Build and smoke test**

```bash
cd bangou && go build -o bangou .

./bangou --db /tmp/test.db
# Open http://localhost:8080 — should see Settings page prompting for input/output dirs
# Configure dirs, then add test files to see them appear as pending
```

- [ ] **Step 3: Cross-compile for Pi**

```bash
GOOS=linux GOARCH=arm64 go build -o bangou-arm64 .
```

- [ ] **Step 4: Commit**

```bash
git add main.go
git commit -m "feat: wire all components, ready for deployment"
```

---

## Task 10: Docker Support

**Files:**
- Create: `bangou/Dockerfile`
- Create: `bangou/docker-compose.yml`

- [ ] **Step 1: Create Dockerfile**

```dockerfile
# bangou/Dockerfile
FROM golang:1.22-alpine AS builder
RUN apk add --no-cache git
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /bangou .

FROM alpine:3.19
RUN apk add --no-cache mkvtoolnix
COPY --from=builder /bangou /usr/local/bin/bangou
ENTRYPOINT ["bangou"]
```

- [ ] **Step 2: Create docker-compose.yml**

```yaml
# bangou/docker-compose.yml
services:
  bangou:
    build: .
    ports:
      - "8080:8080"
    volumes:
      - /mnt/hd1/Download:/input         # writable: merge deletes source parts
      - /mnt/hd1/Multimedia:/output
      - ./data:/data
    command:
      - --db=/data/bangou.db
      - --listen=:8080
    restart: unless-stopped
```

Note: input/output dirs are configured via web UI, not CLI. If using hardlinks, both dirs must be on the same filesystem (auto-detected). Mount volumes accordingly.

- [ ] **Step 3: Test Docker build**

```bash
cd bangou && docker compose build
```

- [ ] **Step 4: Commit**

```bash
git add Dockerfile docker-compose.yml
git commit -m "feat: Docker support with mkvtoolnix"
```

---

## Summary

| Task | Component | Estimated Steps |
|------|-----------|----------------|
| 1 | Bootstrap + Config | 5 |
| 2 | Filename Parser | 6 |
| 3 | SQLite Store | 7 |
| 4 | Directory Scanner | 6 |
| 5 | Pipeline Wiring | 3 |
| 6 | Executor (Link + Merge) | 9 |
| 7 | Web UI | 6 |
| 8 | Link Health Checker | 5 |
| 9 | Full Wiring | 4 |
| 10 | Docker | 4 |

**Out of scope for MVP (future work):**
- MetaTube API integration for metadata/NFO/poster
- Scraping and NFO generation
- aria2 RPC integration (auto-download missing parts)
- Notification (Telegram bot, webhook)
- Multi-user / auth
