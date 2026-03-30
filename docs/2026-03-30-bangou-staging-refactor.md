# Bangou Staging Architecture Refactor

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Refactor bangou to separate staging (in-memory, pre-commit) from committed state (DB, post-commit). Dashboard reflects filesystem reality. DB only records executed results.

**Architecture:** `staging.Manager` holds all pre-commit state in memory, rebuilt from filesystem on startup. `committed.Store` (SQLite) only written after successful link/merge. Restart = rescan + rebuild. No pre-commit persistence.

**This plan supersedes:** The state management portions of `2026-03-30-bangou-mvp.md` (Tasks 3, 5, 7-9) and wiring in `2026-03-30-bangou-scraper.md` (Task 6). Parser, executor, NFO, provider implementations remain valid.

---

## Architecture Overview

```
┌──────────────────────────────────────────────────────┐
│                    IN MEMORY (staging)                 │
│                                                        │
│  scanner ──→ staging.Manager                           │
│              ├── groups: map[string]*StagingGroup       │
│              │   └── number, files[], parsedInfo[],     │
│              │       scrapeResult, userAction            │
│              ├── unknowns: []*UnknownFile               │
│              └── inFlight: map[string]bool (scrape      │
│                  dedup)                                  │
│                                                        │
│  web (Dashboard) ──reads──→ staging.Manager             │
│  web (actions)   ──writes─→ staging.Manager             │
│                              (ignore, manual tag,       │
│                               select parts)             │
└──────────────────────┬───────────────────────────────┘
                       │ user clicks Link/Merge
                       ▼
┌──────────────────────────────────────────────────────┐
│                    SQLITE (committed)                  │
│                                                        │
│  executor writes:                                      │
│  ├── outputs (link_path, link_type, alive)             │
│  ├── merged_parts (consumed source files)              │
│  ├── metadata (NFO content, provider, cover info)      │
│  └── settings (input_dir, output_dir, providers...)    │
│                                                        │
│  web ("In Library") ──reads──→ committed store          │
│  checker ──────────────reads/writes──→ outputs          │
└──────────────────────────────────────────────────────┘
```

---

## Data Models

### Staging (in-memory)

```go
// staging/models.go

type StagingFile struct {
    Path     string
    Filename string
    Size     int64
    Ready    bool      // no .aria2
}

type ParsedFile struct {
    Number string     // ACHJ-057
    Part   int        // 0=single, 1+=part number
    Tags   []string   // 8k, vr
}

type ScrapeResult struct {
    Meta   *provider.MovieMetadata
    Errors map[string]string // provider → error
    Status string            // "", "scraping", "success", "failed"
}

type StagingGroup struct {
    Number  string
    Files   []StagingFile
    Parsed  []ParsedFile    // parallel to Files
    Scrape  ScrapeResult
    Ignored bool            // user chose to ignore (session-only)
}

type UnknownFile struct {
    StagingFile
    Ignored bool
}
```

### Committed (DB)

```sql
-- Only tables needed post-commit

CREATE TABLE IF NOT EXISTS settings (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS outputs (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    number     TEXT NOT NULL,
    link_path  TEXT NOT NULL,
    link_type  TEXT NOT NULL,  -- hardlink, symlink, file (merged)
    alive      BOOLEAN NOT NULL DEFAULT TRUE,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    checked_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_outputs_number ON outputs(number);

CREATE TABLE IF NOT EXISTS merged_parts (
    id       INTEGER PRIMARY KEY AUTOINCREMENT,
    number   TEXT NOT NULL,
    filename TEXT NOT NULL,
    size     INTEGER NOT NULL DEFAULT 0,
    part     INTEGER NOT NULL DEFAULT 0,
    merged_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS metadata (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    number     TEXT UNIQUE NOT NULL,
    title      TEXT NOT NULL DEFAULT '',
    plot       TEXT NOT NULL DEFAULT '',
    director   TEXT NOT NULL DEFAULT '',
    maker      TEXT NOT NULL DEFAULT '',
    label      TEXT NOT NULL DEFAULT '',
    series     TEXT NOT NULL DEFAULT '',
    actors     TEXT NOT NULL DEFAULT '',
    genres     TEXT NOT NULL DEFAULT '',
    cover_url  TEXT NOT NULL DEFAULT '',
    premiered  TEXT NOT NULL DEFAULT '',
    year       TEXT NOT NULL DEFAULT '',
    runtime    TEXT NOT NULL DEFAULT '',
    provider   TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

注意：没有 `source_files`、`parsed_info`、`groups` 表了。这些全在内存。

---

## Lifecycle

### 启动

```
1. Open DB (settings + committed tables)
2. Read settings → get input_dir, output_dir, provider config
3. Create staging.Manager (empty)
4. If input_dir configured:
   a. Full scan input_dir → feed into staging.Manager
   b. For each file: parse → group → auto-scrape (if not in committed)
5. Start fsnotify watcher
6. Start web server
7. Start link health checker (reads committed only)
```

### 文件到达

```
fsnotify CREATE/RENAME
    │
    ▼
scanner.ScanDir → []ScannedFile
    │
    ▼
staging.Manager.Ingest(file)
    ├── parse filename
    ├── parseable? → add to group (by number)
    │               → trigger scrape (if not inFlight and not in committed.metadata)
    └── unparseable? → add to unknowns
```

### 文件消失

```
fsnotify REMOVE
    │
    ▼
staging.Manager.Remove(path)
    ├── find group containing this file → remove file from group
    ├── group now empty? → remove group entirely
    └── was in unknowns? → remove from unknowns
```

### 用户点 Link

```
web POST /api/groups/{number}/link
    │
    ▼
staging.Manager.GetGroup(number)
    │
    ▼
executor.Link(group, outputDir)
    ├── mkdir output/{number}/
    ├── create link (auto-detect hard/sym)
    ├── generate NFO from group.Scrape.Meta (if available)
    ├── download cover from Meta.CoverURL (if available)
    └── on success:
        ├── committed.Store.CreateOutput(...)
        ├── committed.Store.UpsertMetadata(...)
        └── staging.Manager.RemoveGroup(number)
            (group disappears from Dashboard)
```

### 用户点 Merge

```
web POST /api/groups/{number}/merge
    │
    ▼
staging.Manager.GetGroup(number)
    │
    ▼
executor.Merge(group, outputDir)
    ├── mkvmerge parts → output/{number}/{number}.mkv
    ├── generate NFO + download cover
    ├── delete source parts from input dir
    └── on success:
        ├── committed.Store.CreateOutput(...)
        ├── committed.Store.RecordMergedParts(...)
        ├── committed.Store.UpsertMetadata(...)
        └── staging.Manager.RemoveGroup(number)
```

### 重启

```
全部 staging 状态丢失 → 重新扫描 input_dir → 重建
已提交的 outputs 在 DB 里 → "In Library" 页面不受影响
源文件已被 merge 删除的 → 不会再出现在 input_dir → 不会重建到 staging
已 link 的文件仍在 input_dir → 重扫时出现 → 但检查 committed.outputs 发现已入库 → 跳过
```

---

## File Structure

```
bangou/
├── staging/
│   ├── manager.go          # Manager struct, Ingest/Remove/GetGroup/List
│   ├── manager_test.go
│   └── models.go           # StagingGroup, StagingFile, UnknownFile
│
├── committed/
│   ├── store.go            # Store interface (outputs, merged_parts, metadata, settings)
│   ├── sqlite.go           # SQLite implementation
│   ├── sqlite_test.go
│   └── migrations.go       # Schema (only post-commit tables)
│
├── parser/                 # unchanged from MVP plan
├── provider/               # unchanged from scraper plan
├── nfo/                    # unchanged from scraper plan
├── executor/
│   ├── executor.go         # Link/Merge, reads staging, writes committed
│   └── executor_test.go
├── scanner/                # unchanged, but only feeds staging.Manager
├── checker/                # unchanged, reads committed.outputs
├── web/
│   ├── server.go
│   ├── handlers.go         # Dashboard reads staging; Library reads committed
│   └── templates/
├── config/                 # --db, --listen only
└── main.go
```

---

## Task 1: Staging Manager

**Files:**
- Create: `bangou/staging/models.go`
- Create: `bangou/staging/manager.go`
- Create: `bangou/staging/manager_test.go`

- [ ] **Step 1: Write models**

```go
// staging/models.go
package staging

import "github.com/CheerChen/bangou/provider"

type StagingFile struct {
	Path     string
	Filename string
	Size     int64
	Ready    bool
}

type ParsedFile struct {
	Number string
	Part   int
	Tags   []string
}

type ScrapeResult struct {
	Meta   *provider.MovieMetadata
	Errors map[string]string
	Status string // "", "scraping", "success", "failed"
}

type StagingGroup struct {
	Number  string
	Files   []StagingFile
	Parsed  []ParsedFile
	Scrape  ScrapeResult
	Ignored bool
}

type UnknownFile struct {
	StagingFile
	Ignored bool
}
```

- [ ] **Step 2: Write failing tests**

```go
// staging/manager_test.go
package staging

import (
	"testing"

	"github.com/CheerChen/bangou/parser"
)

func TestIngestAndGroup(t *testing.T) {
	m := New(nil) // nil committed store for now

	m.Ingest(StagingFile{Path: "/input/ACHJ-057.mp4", Filename: "ACHJ-057.mp4", Size: 1024, Ready: true})

	groups := m.ListGroups()
	if len(groups) != 1 {
		t.Fatalf("got %d groups, want 1", len(groups))
	}
	if groups[0].Number != "ACHJ-057" {
		t.Errorf("number = %q, want ACHJ-057", groups[0].Number)
	}
}

func TestIngestMultipleParts(t *testing.T) {
	m := New(nil)

	m.Ingest(StagingFile{Path: "/input/sivr476_1_8k.mp4", Filename: "sivr476_1_8k.mp4", Size: 5000, Ready: true})
	m.Ingest(StagingFile{Path: "/input/sivr476_3_8k.mp4", Filename: "sivr476_3_8k.mp4", Size: 5000, Ready: true})

	groups := m.ListGroups()
	if len(groups) != 1 {
		t.Fatalf("got %d groups, want 1", len(groups))
	}
	if len(groups[0].Files) != 2 {
		t.Errorf("got %d files, want 2", len(groups[0].Files))
	}
}

func TestIngestUnknown(t *testing.T) {
	m := New(nil)

	m.Ingest(StagingFile{Path: "/input/random.mp4", Filename: "random.mp4", Size: 100, Ready: true})

	groups := m.ListGroups()
	unknowns := m.ListUnknowns()
	if len(groups) != 0 {
		t.Errorf("got %d groups, want 0", len(groups))
	}
	if len(unknowns) != 1 {
		t.Errorf("got %d unknowns, want 1", len(unknowns))
	}
}

func TestRemoveFile(t *testing.T) {
	m := New(nil)

	m.Ingest(StagingFile{Path: "/input/ACHJ-057.mp4", Filename: "ACHJ-057.mp4", Size: 1024, Ready: true})
	m.Remove("/input/ACHJ-057.mp4")

	if len(m.ListGroups()) != 0 {
		t.Error("group should be removed when last file removed")
	}
}

func TestSkipAlreadyCommitted(t *testing.T) {
	// Files whose number already exists in committed.outputs should still appear
	// in staging (user might want to re-merge with new parts), but should NOT
	// trigger auto-scrape.
	// This test verifies the file is ingested, not that scrape is skipped
	// (scrape triggering is tested at integration level).
	m := New(nil)
	m.Ingest(StagingFile{Path: "/input/ACHJ-057.mp4", Filename: "ACHJ-057.mp4", Size: 1024, Ready: true})

	groups := m.ListGroups()
	if len(groups) != 1 {
		t.Fatalf("committed files should still appear in staging")
	}
}
```

- [ ] **Step 3: Run tests to verify they fail**

```bash
cd bangou && go test ./staging/ -v
```

- [ ] **Step 4: Implement Manager**

```go
// staging/manager.go
package staging

import (
	"sync"

	"github.com/CheerChen/bangou/parser"
)

type Manager struct {
	mu        sync.RWMutex
	groups    map[string]*StagingGroup  // number → group
	unknowns  map[string]*UnknownFile   // path → unknown
	fileIndex map[string]string         // path → number (for removal lookup)

	// Callback: called when a new number enters staging and needs scraping.
	// Set by main.go to wire scrape logic without circular deps.
	OnNewNumber func(number string)
}

func New() *Manager {
	return &Manager{
		groups:    make(map[string]*StagingGroup),
		unknowns:  make(map[string]*UnknownFile),
		fileIndex: make(map[string]string),
	}
}

func (m *Manager) Ingest(f StagingFile) {
	if !f.Ready {
		return // still downloading, ignore
	}

	parsed := parser.Parse(f.Filename)

	m.mu.Lock()
	defer m.mu.Unlock()

	if parsed.Number == "" {
		// Unknown file
		if _, exists := m.unknowns[f.Path]; !exists {
			m.unknowns[f.Path] = &UnknownFile{StagingFile: f}
		}
		return
	}

	// Add to group
	g, exists := m.groups[parsed.Number]
	if !exists {
		g = &StagingGroup{Number: parsed.Number}
		m.groups[parsed.Number] = g
	}

	// Deduplicate by path
	for _, existing := range g.Files {
		if existing.Path == f.Path {
			return
		}
	}

	g.Files = append(g.Files, f)
	g.Parsed = append(g.Parsed, ParsedFile{
		Number: parsed.Number,
		Part:   parsed.Part,
		Tags:   parsed.Tags,
	})
	m.fileIndex[f.Path] = parsed.Number

	// Trigger scrape for new number
	if !exists && m.OnNewNumber != nil {
		go m.OnNewNumber(parsed.Number)
	}
}

func (m *Manager) Remove(path string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check unknowns
	delete(m.unknowns, path)

	// Check groups
	number, ok := m.fileIndex[path]
	if !ok {
		return
	}
	delete(m.fileIndex, path)

	g, ok := m.groups[number]
	if !ok {
		return
	}

	// Remove file from group
	for i, f := range g.Files {
		if f.Path == path {
			g.Files = append(g.Files[:i], g.Files[i+1:]...)
			g.Parsed = append(g.Parsed[:i], g.Parsed[i+1:]...)
			break
		}
	}

	// Remove group if empty
	if len(g.Files) == 0 {
		delete(m.groups, number)
	}
}

func (m *Manager) RemoveGroup(number string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	g, ok := m.groups[number]
	if !ok {
		return
	}
	for _, f := range g.Files {
		delete(m.fileIndex, f.Path)
	}
	delete(m.groups, number)
}

func (m *Manager) GetGroup(number string) *StagingGroup {
	m.mu.RLock()
	defer m.mu.RUnlock()
	g := m.groups[number]
	if g == nil {
		return nil
	}
	// Return a copy to avoid race
	cpy := *g
	cpy.Files = append([]StagingFile{}, g.Files...)
	cpy.Parsed = append([]ParsedFile{}, g.Parsed...)
	return &cpy
}

func (m *Manager) ListGroups() []StagingGroup {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []StagingGroup
	for _, g := range m.groups {
		if !g.Ignored {
			cpy := *g
			result = append(result, cpy)
		}
	}
	return result
}

func (m *Manager) ListUnknowns() []UnknownFile {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []UnknownFile
	for _, u := range m.unknowns {
		if !u.Ignored {
			result = append(result, *u)
		}
	}
	return result
}

func (m *Manager) SetScrapeResult(number string, result ScrapeResult) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if g, ok := m.groups[number]; ok {
		g.Scrape = result
	}
}

func (m *Manager) SetIgnored(number string, ignored bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if g, ok := m.groups[number]; ok {
		g.Ignored = ignored
	}
}

func (m *Manager) SetUnknownIgnored(path string, ignored bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if u, ok := m.unknowns[path]; ok {
		u.Ignored = ignored
	}
}

// ManualTag moves an unknown file into a group with the given number.
func (m *Manager) ManualTag(path string, number string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	u, ok := m.unknowns[path]
	if !ok {
		return
	}
	delete(m.unknowns, path)

	g, exists := m.groups[number]
	if !exists {
		g = &StagingGroup{Number: number}
		m.groups[number] = g
	}

	g.Files = append(g.Files, u.StagingFile)
	g.Parsed = append(g.Parsed, ParsedFile{Number: number})
	m.fileIndex[path] = number

	if !exists && m.OnNewNumber != nil {
		go m.OnNewNumber(number)
	}
}
```

- [ ] **Step 5: Run tests**

```bash
cd bangou && go test ./staging/ -v
```

- [ ] **Step 6: Commit**

```bash
git add staging/
git commit -m "feat: in-memory staging manager for pre-commit state"
```

---

## Task 2: Simplify Committed Store

Replace the over-specified store with only post-commit tables.

**Files:**
- Rewrite: `bangou/committed/store.go` (was `store/store.go`)
- Rewrite: `bangou/committed/sqlite.go` (was `store/sqlite.go`)
- Rewrite: `bangou/committed/migrations.go`
- Create: `bangou/committed/sqlite_test.go`

- [ ] **Step 1: Define simplified interface**

```go
// committed/store.go
package committed

import "context"

type Store interface {
	// Outputs
	CreateOutput(ctx context.Context, o *Output) error
	ListOutputsByNumber(ctx context.Context, number string) ([]Output, error)
	ListAllOutputs(ctx context.Context) ([]Output, error)
	SetOutputAlive(ctx context.Context, id int64, alive bool) error
	ListOrphanedOutputs(ctx context.Context) ([]Output, error)
	IsCommitted(ctx context.Context, number string) (bool, error)

	// Merged parts
	RecordMergedParts(ctx context.Context, parts []MergedPart) error
	GetMergedParts(ctx context.Context, number string) ([]MergedPart, error)

	// Metadata (written at commit time)
	UpsertMetadata(ctx context.Context, m *Metadata) error
	GetMetadata(ctx context.Context, number string) (*Metadata, error)

	// Settings
	GetSetting(ctx context.Context, key string) (string, error)
	SetSetting(ctx context.Context, key, value string) error
	GetAllSettings(ctx context.Context) (map[string]string, error)

	Close() error
}
```

- [ ] **Step 2: Define models**

```go
// committed/models.go
package committed

import "time"

type Output struct {
	ID        int64
	Number    string
	LinkPath  string
	LinkType  string
	Alive     bool
	CreatedAt time.Time
	CheckedAt time.Time
}

type MergedPart struct {
	Number   string
	Filename string
	Size     int64
	Part     int
}

type Metadata struct {
	ID        int64
	Number    string
	Title     string
	Plot      string
	Director  string
	Maker     string
	Label     string
	Series    string
	Actors    string // comma-separated
	Genres    string // comma-separated
	CoverURL  string
	Premiered string
	Year      string
	Runtime   string
	Provider  string
	CreatedAt time.Time
	UpdatedAt time.Time
}
```

- [ ] **Step 3: Write migrations (post-commit tables only)**

Use the schema from the "Data Models" section above.

- [ ] **Step 4: Implement SQLite store**

Standard implementation of the interface. All methods are straightforward CRUD. The key new method:

```go
func (s *SQLiteStore) IsCommitted(ctx context.Context, number string) (bool, error) {
	var count int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM outputs WHERE number = ? AND alive = TRUE`, number).Scan(&count)
	return count > 0, err
}
```

- [ ] **Step 5: Write tests, verify**

```bash
cd bangou && go test ./committed/ -v
```

- [ ] **Step 6: Commit**

```bash
git add committed/
git commit -m "feat: simplified committed store (post-commit only)"
```

---

## Task 3: Rewire Main + Executor

Connect staging manager, scanner, scrape queue, and executor.

**Files:**
- Rewrite: `bangou/main.go`
- Modify: `bangou/executor/executor.go`

- [ ] **Step 1: Rewrite main.go**

```go
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"time"

	"github.com/CheerChen/bangou/checker"
	"github.com/CheerChen/bangou/committed"
	"github.com/CheerChen/bangou/config"
	"github.com/CheerChen/bangou/executor"
	"github.com/CheerChen/bangou/provider"
	"github.com/CheerChen/bangou/scanner"
	"github.com/CheerChen/bangou/staging"
	"github.com/CheerChen/bangou/web"
)

func main() {
	cfg := config.Parse()

	db, err := committed.NewSQLite(cfg.DBPath)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	// Staging manager
	mgr := staging.New()

	// Scrape queue (single consumer, inFlight dedup)
	scrapeQueue := make(chan string, 100)
	var inFlight sync.Map

	mgr.OnNewNumber = func(number string) {
		// Skip if already committed (has live output)
		if ok, _ := db.IsCommitted(ctx, number); ok {
			return
		}
		// Dedup
		if _, loaded := inFlight.LoadOrStore(number, true); loaded {
			return
		}
		scrapeQueue <- number
	}

	// Scrape consumer
	go func() {
		for number := range scrapeQueue {
			providers := buildProviders(ctx, db)
			result := provider.Chain(ctx, providers, number)
			mgr.SetScrapeResult(number, staging.ScrapeResult{
				Meta:   result.Meta,
				Errors: result.Errors,
				Status: map[bool]string{true: "success", false: "failed"}[result.Meta != nil],
			})
			inFlight.Delete(number)
		}
	}()

	// Read settings
	settings, _ := db.GetAllSettings(ctx)
	inputDir := settings["input_dir"]
	outputDir := settings["output_dir"]

	// Executor
	exec := executor.New(db, mgr, outputDir)

	// Initial scan + watch
	if inputDir != "" {
		events := make(chan scanner.Event, 100)
		if err := scanner.Watch(ctx, inputDir, events); err != nil {
			log.Printf("warn: watch failed: %v", err)
		}
		// Feed scanner events into staging
		go func() {
			for ev := range events {
				mgr.Ingest(staging.StagingFile{
					Path:     ev.File.Path,
					Filename: ev.File.Filename,
					Size:     ev.File.Size,
					Ready:    ev.File.Ready,
				})
			}
		}()
		log.Printf("watching %s", inputDir)
	} else {
		log.Printf("no input directory configured — open web UI to set up")
	}

	// Link health checker
	go checker.Run(ctx, db, 1*time.Hour)

	// Web server
	srv := web.NewServer(mgr, db, exec)
	log.Printf("web UI: http://%s", cfg.ListenAddr)
	if err := http.ListenAndServe(cfg.ListenAddr, srv); err != nil {
		log.Fatal(err)
	}
}

func buildProviders(ctx context.Context, db committed.Store) []provider.Provider {
	settings, _ := db.GetAllSettings(ctx)
	order := settings["provider_order"]
	if order == "" {
		order = "avwiki,dmm"
	}
	var providers []provider.Provider
	for _, name := range strings.Split(order, ",") {
		name = strings.TrimSpace(name)
		if settings["provider_enabled_"+name] == "false" {
			continue
		}
		switch name {
		case "avwiki":
			providers = append(providers, provider.NewAVWiki())
		case "dmm":
			apiID := settings["dmm_api_id"]
			affID := settings["dmm_affiliate_id"]
			if apiID != "" && affID != "" {
				providers = append(providers, provider.NewDMM(apiID, affID))
			}
		}
	}
	return providers
}
```

- [ ] **Step 2: Update executor to read staging, write committed**

```go
// executor/executor.go
package executor

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/CheerChen/bangou/committed"
	"github.com/CheerChen/bangou/nfo"
	"github.com/CheerChen/bangou/provider"
	"github.com/CheerChen/bangou/staging"
)

type Executor struct {
	store     committed.Store
	staging   *staging.Manager
	outputDir string
}

func New(store committed.Store, stg *staging.Manager, outputDir string) *Executor {
	return &Executor{store: store, staging: stg, outputDir: outputDir}
}

func (e *Executor) Link(ctx context.Context, number string) error {
	group := e.staging.GetGroup(number)
	if group == nil {
		return fmt.Errorf("group %s not found in staging", number)
	}
	if len(group.Files) == 0 {
		return fmt.Errorf("no files in group %s", number)
	}

	src := group.Files[0]
	outDir := filepath.Join(e.outputDir, number)
	os.MkdirAll(outDir, 0755)

	linkType := detectLinkType(filepath.Dir(src.Path), e.outputDir)
	linkPath, err := CreateLink(src.Path, outDir, number, linkType)
	if err != nil {
		return err
	}

	// Write NFO + cover from scrape result
	e.writeMetadata(ctx, number, outDir, group.Scrape.Meta)

	// Commit to DB
	e.store.CreateOutput(ctx, &committed.Output{
		Number:   number,
		LinkPath: linkPath,
		LinkType: linkType,
	})
	if group.Scrape.Meta != nil {
		e.commitMetadata(ctx, number, group.Scrape.Meta)
	}

	// Remove from staging
	e.staging.RemoveGroup(number)
	log.Printf("linked: %s → %s", number, linkPath)
	return nil
}

func (e *Executor) Merge(ctx context.Context, number string) error {
	group := e.staging.GetGroup(number)
	if group == nil || len(group.Files) < 2 {
		return fmt.Errorf("need 2+ files to merge %s", number)
	}

	outDir := filepath.Join(e.outputDir, number)
	os.MkdirAll(outDir, 0755)
	mergedPath := filepath.Join(outDir, number+".mkv")

	var parts []string
	for _, f := range group.Files {
		parts = append(parts, f.Path)
	}

	if err := MergeFiles(parts, mergedPath); err != nil {
		return err
	}

	// Write NFO + cover
	e.writeMetadata(ctx, number, outDir, group.Scrape.Meta)

	// Record merged parts + delete source files
	var mergedParts []committed.MergedPart
	for i, f := range group.Files {
		part := 0
		if i < len(group.Parsed) {
			part = group.Parsed[i].Part
		}
		mergedParts = append(mergedParts, committed.MergedPart{
			Number: number, Filename: f.Filename, Size: f.Size, Part: part,
		})
		os.Remove(f.Path) // fsnotify will fire Remove → staging auto-cleans
	}

	// Commit to DB
	e.store.CreateOutput(ctx, &committed.Output{
		Number:   number,
		LinkPath: mergedPath,
		LinkType: "file",
	})
	e.store.RecordMergedParts(ctx, mergedParts)
	if group.Scrape.Meta != nil {
		e.commitMetadata(ctx, number, group.Scrape.Meta)
	}

	// staging auto-cleans via fsnotify Remove events (source files deleted)
	// but also explicitly remove in case fsnotify is slow
	e.staging.RemoveGroup(number)
	log.Printf("merged %d parts → %s", len(parts), mergedPath)
	return nil
}

func (e *Executor) writeMetadata(ctx context.Context, number, outDir string, meta *provider.MovieMetadata) {
	if meta == nil {
		return
	}
	nfoPath := filepath.Join(outDir, number+".nfo")
	nfo.Save(meta, nfoPath)
	provider.DownloadCover(ctx, meta.CoverURL, outDir, number)
}

func (e *Executor) commitMetadata(ctx context.Context, number string, meta *provider.MovieMetadata) {
	e.store.UpsertMetadata(ctx, &committed.Metadata{
		Number:    number,
		Title:     meta.Title,
		Plot:      meta.Plot,
		Director:  meta.Director,
		Maker:     meta.Maker,
		Label:     meta.Label,
		Series:    meta.Series,
		Actors:    strings.Join(meta.Actors, ","),
		Genres:    strings.Join(meta.Genres, ","),
		CoverURL:  meta.CoverURL,
		Premiered: meta.Premiered,
		Year:      meta.Year,
		Runtime:   meta.Runtime,
		Provider:  meta.Provider,
	})
}
```

- [ ] **Step 3: Commit**

```bash
git add main.go executor/ committed/ staging/
git commit -m "feat: staging/committed split, rewire main and executor"
```

---

## Task 4: Update Web Handlers

Dashboard reads staging. "In Library" reads committed.

**Files:**
- Rewrite: `bangou/web/handlers.go`
- Modify: `bangou/web/server.go`

- [ ] **Step 1: Update handlers**

```go
// web/handlers.go (key changes)

type Handlers struct {
	staging  *staging.Manager
	store    committed.Store
	executor *executor.Executor
}

func (h *Handlers) Index(w http.ResponseWriter, r *http.Request) {
	// Pending: from staging
	groups := h.staging.ListGroups()
	unknowns := h.staging.ListUnknowns()

	// In Library: from committed DB
	outputs, _ := h.store.ListAllOutputs(r.Context())

	data := map[string]interface{}{
		"Pending":  groups,   // []staging.StagingGroup — live from memory
		"Unknown":  unknowns, // []staging.UnknownFile
		"Library":  outputs,  // []committed.Output — from DB
	}
	renderTemplate(w, "layout.html", data)
}

func (h *Handlers) GroupAction(w http.ResponseWriter, r *http.Request) {
	number := r.PathValue("number")
	action := r.FormValue("action")

	switch action {
	case "link":
		if err := h.executor.Link(r.Context(), number); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
	case "merge":
		if err := h.executor.Merge(r.Context(), number); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
	case "ignore":
		h.staging.SetIgnored(number, true)
	case "rescrape":
		if h.staging.OnNewNumber != nil {
			go h.staging.OnNewNumber(number)
		}
	}

	// Return updated page via htmx or redirect
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
```

- [ ] **Step 2: Update routes**

```go
// web/server.go

func NewServer(stg *staging.Manager, store committed.Store, exec *executor.Executor) http.Handler {
	h := &Handlers{staging: stg, store: store, executor: exec}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /", h.Index)
	mux.HandleFunc("GET /settings", h.Settings)
	mux.HandleFunc("POST /api/settings", h.SaveSettings)
	mux.HandleFunc("POST /api/groups/{number}/action", h.GroupAction)
	mux.HandleFunc("POST /api/files/{path}/tag", h.ManualTag)
	mux.HandleFunc("POST /api/scan", h.TriggerScan)
	mux.Handle("GET /static/", http.FileServerFS(content))
	return mux
}
```

- [ ] **Step 3: Commit**

```bash
git add web/
git commit -m "feat: web handlers read staging for dashboard, committed for library"
```

---

## Task 5: Remove Pre-Commit DB Tables

Delete or ignore the old `source_files`, `parsed_info`, `groups` tables. If migrating from existing DB, drop them.

- [ ] **Step 1: Verify no code references old tables**

```bash
grep -r "source_files\|parsed_info\|\"groups\"" bangou/ --include="*.go"
```

Expected: no hits in committed/ or staging/.

- [ ] **Step 2: Clean up migrations if old tables exist**

```go
// Add to committed/migrations.go startup
const cleanupOldTables = `
DROP TABLE IF EXISTS source_files;
DROP TABLE IF EXISTS parsed_info;
DROP TABLE IF EXISTS groups;
`
```

- [ ] **Step 3: Commit**

```bash
git add committed/
git commit -m "refactor: remove pre-commit DB tables (source_files, parsed_info, groups)"
```

---

## Summary

| Task | What | Key Change |
|------|------|-----------|
| 1 | Staging Manager | In-memory groups, unknowns, scrape results. Rebuilt on restart. |
| 2 | Simplified Committed Store | Only outputs, merged_parts, metadata, settings. No pre-commit state. |
| 3 | Rewire Main + Executor | Scanner → staging, executor reads staging writes committed. |
| 4 | Update Web Handlers | Dashboard reads staging (live), Library reads committed (DB). |
| 5 | Remove old tables | Drop source_files, parsed_info, groups. |

**What stays unchanged from previous plans:**
- `parser/` — filename parsing logic
- `provider/` — AVWiki, DMM, MetaTube stub
- `nfo/` — NFO generation
- `scanner/` — fsnotify + ScanDir
- `checker/` — link health check (reads committed.outputs)
- `config/` — CLI flags
- `web/templates/` — HTML templates (minor updates to read from staging types)

**Key behavioral changes:**
- Restart = staging empty → full rescan → groups rebuild → scrape retriggers
- Ignore is session-only, lost on restart
- Manual tag is session-only, lost on restart
- DB is clean — only contains things that actually happened
- Dashboard is always a live view of the filesystem
