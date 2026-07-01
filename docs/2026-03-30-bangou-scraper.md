# Bangou Scraper & NFO Generation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add metadata scraping (av-wiki.net + DMM API) and NFO/cover generation to bangou, triggered automatically after filename parsing. Designed for multiple sources with configurable priority order.

**Architecture:** Provider interface with pluggable implementations. AVWiki (no config needed) + DMM API (needs api_id + affiliate_id) as MVP providers. Provider order configurable in Settings UI. Scrape triggers automatically when a file enters `parsed` state. Results cached in DB. NFO + cover written to output dir alongside the link/merged file. Failure = silent skip, no blocking.

**Tech Stack:** goquery (HTML parsing), net/http, encoding/xml (NFO generation)

---

## Data Flow

```
handleEvent (existing)
    │
    ▼
file parsed, group ensured
    │
    ▼ NEW: auto-scrape
    │
┌───┴────────────────────────────┐
│  Has metadata in DB for this   │
│  group.number already?         │
├──── yes ──→ skip               │
├──── no  ──→ scrape             │
│              │                 │
│         ┌────┴─────┐          │
│         │ Provider │          │
│         │ chain    │          │
│         └────┬─────┘          │
│              │                │
│         ┌────┴──────┐        │
│         │ (order    │        │
│         │ from      │        │
│         │ Settings) │        │
│         │           │        │
│         │ 1. AVWiki │ (MVP)  │
│         │ 2. DMM API│ (MVP)  │
│         │ 3.MetaTube│(future)│
│         └────┬──────┘        │
│              │               │
│         success? ─── no ──→ done (metadata stays empty)
│              │                │
│             yes               │
│              │                │
│         save to DB            │
│         download cover        │
└───────────────────────────────┘
```

NFO 不在刮削时生成 — 在用户点 "link" 或 "merge" 执行入库时，从 DB 元数据生成 NFO + 写入 cover。这样：
- 刮削失败不阻塞分组和决策
- 元数据可以在 UI 上预览/编辑后再写入 NFO
- 多次刮削（换源）不会产生多个 NFO 文件

---

## File Structure (新增)

```
bangou/
├── provider/
│   ├── provider.go          # Provider 接口 + MovieMetadata 结构体 + Chain
│   ├── avwiki.go            # av-wiki.net 实现（无需配置）
│   ├── avwiki_test.go
│   ├── dmm.go               # DMM Affiliate API 实现（需 api_id + affiliate_id）
│   ├── dmm_test.go
│   └── metatube.go          # (future) MetaTube API 实现
│
├── nfo/
│   ├── nfo.go               # MovieMetadata → Emby NFO XML
│   └── nfo_test.go
│
├── store/
│   ├── models.go            # 新增 MovieMetadata model
│   ├── migrations.go        # 新增 metadata 表
│   └── sqlite.go            # 新增 metadata CRUD
│
└── web/
    └── templates/
        └── index.html       # group card 展示元数据预览
```

---

## Task 1: Provider Interface + MovieMetadata Model

**Files:**
- Create: `bangou/provider/provider.go`
- Modify: `bangou/store/models.go`
- Modify: `bangou/store/migrations.go`
- Modify: `bangou/store/store.go`
- Modify: `bangou/store/sqlite.go`

- [ ] **Step 1: Define provider interface and metadata model**

```go
// provider/provider.go
package provider

import "context"

// MovieMetadata is the unified result from any provider.
type MovieMetadata struct {
	Number    string   // ACHJ-057
	Title     string
	Plot      string
	Director  string
	Maker     string   // studio
	Label     string
	Series    string
	Actors    []string
	Genres    []string
	CoverURL  string
	Premiered string   // YYYY-MM-DD
	Year      string   // YYYY
	Runtime   string
	Provider  string   // "avwiki", "metatube", "dmm"
}

// Provider fetches metadata for a given serial number.
type Provider interface {
	// Name returns the provider identifier.
	Name() string
	// Scrape fetches metadata for the given number. Returns nil if not found.
	Scrape(ctx context.Context, number string) (*MovieMetadata, error)
}

// Chain tries providers in order, returns first success.
func Chain(ctx context.Context, providers []Provider, number string) *MovieMetadata {
	for _, p := range providers {
		meta, err := p.Scrape(ctx, number)
		if err == nil && meta != nil {
			meta.Provider = p.Name()
			return meta
		}
	}
	return nil
}
```

- [ ] **Step 2: Add metadata table to migrations**

```sql
-- Add to store/migrations.go schemaV1

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
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

- [ ] **Step 3: Add metadata model to store**

```go
// Add to store/models.go

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
}
```

- [ ] **Step 4: Add metadata methods to Store interface**

```go
// Add to store/store.go

	// Metadata
	GetMetadata(ctx context.Context, number string) (*Metadata, error)
	UpsertMetadata(ctx context.Context, m *Metadata) error
```

- [ ] **Step 5: Implement metadata store methods**

```go
// Add to store/sqlite.go

func (s *SQLiteStore) GetMetadata(ctx context.Context, number string) (*Metadata, error) {
	m := &Metadata{}
	err := s.db.QueryRowContext(ctx,
		`SELECT id, number, title, plot, director, maker, label, series,
		        actors, genres, cover_url, premiered, year, runtime, provider, created_at
		 FROM metadata WHERE number = ?`, number).
		Scan(&m.ID, &m.Number, &m.Title, &m.Plot, &m.Director, &m.Maker,
			&m.Label, &m.Series, &m.Actors, &m.Genres, &m.CoverURL,
			&m.Premiered, &m.Year, &m.Runtime, &m.Provider, &m.CreatedAt)
	if err != nil {
		return nil, err
	}
	return m, nil
}

func (s *SQLiteStore) UpsertMetadata(ctx context.Context, m *Metadata) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO metadata (number, title, plot, director, maker, label, series,
		                       actors, genres, cover_url, premiered, year, runtime, provider)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(number) DO UPDATE SET
		   title=excluded.title, plot=excluded.plot, director=excluded.director,
		   maker=excluded.maker, label=excluded.label, series=excluded.series,
		   actors=excluded.actors, genres=excluded.genres, cover_url=excluded.cover_url,
		   premiered=excluded.premiered, year=excluded.year, runtime=excluded.runtime,
		   provider=excluded.provider`,
		m.Number, m.Title, m.Plot, m.Director, m.Maker, m.Label, m.Series,
		m.Actors, m.Genres, m.CoverURL, m.Premiered, m.Year, m.Runtime, m.Provider)
	return err
}
```

- [ ] **Step 6: Commit**

```bash
git add provider/ store/
git commit -m "feat: provider interface, metadata model and store"
```

---

## Task 2: AVWiki Provider

Ported from `pikpak-batch-renamer.user.js` and `pkg/scraper/avwiki.go`. Two strategies:
1. Direct URL access: `https://av-wiki.net/{number}/`
2. Fallback search: `https://av-wiki.net/?s={number}&post_type=product`

**Files:**
- Create: `bangou/provider/avwiki.go`
- Create: `bangou/provider/avwiki_test.go`

- [ ] **Step 1: Write failing test**

```go
// provider/avwiki_test.go
package provider

import (
	"context"
	"testing"
)

func TestAVWikiScrape(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	p := NewAVWiki()
	ctx := context.Background()

	// Known番号, should succeed
	meta, err := p.Scrape(ctx, "ACHJ-057")
	if err != nil {
		t.Fatalf("Scrape failed: %v", err)
	}
	if meta == nil {
		t.Fatal("expected metadata, got nil")
	}
	if meta.Title == "" {
		t.Error("expected non-empty title")
	}
	if meta.Number != "ACHJ-057" {
		t.Errorf("Number = %q, want ACHJ-057", meta.Number)
	}

	t.Logf("title=%s premiered=%s actors=%v maker=%s", meta.Title, meta.Premiered, meta.Actors, meta.Maker)
}

func TestAVWikiScrapeNotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	p := NewAVWiki()
	ctx := context.Background()

	meta, err := p.Scrape(ctx, "ZZZZZ-99999")
	if err == nil && meta != nil {
		t.Error("expected nil for non-existent number")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd bangou && go test ./provider/ -v -run TestAVWiki
```

Expected: FAIL — `NewAVWiki` not defined.

- [ ] **Step 3: Implement AVWiki provider**

```go
// provider/avwiki.go
package provider

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

const (
	avwikiDirectURL = "https://av-wiki.net/%s/"
	avwikiSearchURL = "https://av-wiki.net/?s=%s&post_type=product"
)

type AVWiki struct {
	client *http.Client
}

func NewAVWiki() *AVWiki {
	return &AVWiki{
		client: &http.Client{Timeout: 15 * time.Second},
	}
}

func (a *AVWiki) Name() string { return "avwiki" }

func (a *AVWiki) Scrape(ctx context.Context, number string) (*MovieMetadata, error) {
	// Strategy 1: Direct URL access
	doc, err := a.fetchDoc(ctx, fmt.Sprintf(avwikiDirectURL, strings.ToLower(number)))
	if err == nil {
		if meta := a.parseDetail(doc, number); meta != nil {
			return meta, nil
		}
	}

	// Strategy 2: Search fallback
	doc, err = a.fetchDoc(ctx, fmt.Sprintf(avwikiSearchURL, number))
	if err != nil {
		return nil, fmt.Errorf("avwiki search failed: %w", err)
	}

	detailURL := a.findFirstResult(doc, number)
	if detailURL == "" {
		return nil, fmt.Errorf("avwiki: no results for %s", number)
	}

	doc, err = a.fetchDoc(ctx, detailURL)
	if err != nil {
		return nil, fmt.Errorf("avwiki detail fetch failed: %w", err)
	}

	meta := a.parseDetail(doc, number)
	if meta == nil {
		return nil, fmt.Errorf("avwiki: failed to parse detail for %s", number)
	}
	return meta, nil
}

func (a *AVWiki) fetchDoc(ctx context.Context, url string) (*goquery.Document, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	return goquery.NewDocumentFromReader(resp.Body)
}

func (a *AVWiki) findFirstResult(doc *goquery.Document, number string) string {
	series := strings.Split(strings.ToLower(number), "-")[0]
	var result string
	doc.Find(".read-more a").EachWithBreak(func(i int, s *goquery.Selection) bool {
		href, exists := s.Attr("href")
		if exists && strings.Contains(strings.ToLower(href), series) {
			result = href
			return false
		}
		return true
	})
	return result
}

func (a *AVWiki) parseDetail(doc *goquery.Document, number string) *MovieMetadata {
	title := strings.TrimSpace(doc.Find(".blockquote-like p").Text())
	if title == "" {
		// Fallback: try image alt
		title, _ = doc.Find(".article-thumbnail a img").Attr("alt")
		title = strings.TrimSpace(title)
	}
	if title == "" {
		return nil
	}

	meta := &MovieMetadata{
		Number: number,
		Title:  title,
	}

	// Cover image
	meta.CoverURL, _ = doc.Find(".article-thumbnail a img").Attr("src")

	// Release date
	meta.Premiered = strings.TrimSpace(
		doc.Find("dl.dltable dt:contains('配信開始日')").First().Next().Text())
	if meta.Premiered != "" && len(meta.Premiered) >= 4 {
		meta.Year = meta.Premiered[:4]
	}

	// Director
	meta.Director = strings.TrimSpace(
		doc.Find("span[itemprop=director]").First().Text())

	// Maker (studio)
	meta.Maker = strings.TrimSpace(
		doc.Find("dl.dltable dt:contains('メーカー')").First().Next().Text())

	// Label
	meta.Label = strings.TrimSpace(
		doc.Find("dl.dltable dt:contains('レーベル')").First().Next().Text())

	// Actors
	doc.Find("dl.dltable dt:contains('AV女優名')").First().Next().Find("a").Each(
		func(i int, s *goquery.Selection) {
			name := strings.TrimSpace(s.Text())
			if name != "" {
				meta.Actors = append(meta.Actors, name)
			}
		})

	// Genres/tags
	doc.Find("div.cat-link a").Each(func(i int, s *goquery.Selection) {
		tag := strings.TrimSpace(s.Text())
		if tag != "" {
			meta.Genres = append(meta.Genres, tag)
		}
	})

	return meta
}
```

- [ ] **Step 4: Run integration test**

```bash
cd bangou && go get github.com/PuerkitoBio/goquery
cd bangou && go test ./provider/ -v -run TestAVWiki
```

Expected: PASS. Logs show title, premiered, actors, maker for ACHJ-057.

- [ ] **Step 5: Commit**

```bash
git add provider/
git commit -m "feat: AVWiki provider with direct access + search fallback"
```

---

## Task 3: DMM API Provider

DMM Affiliate API v3. Requires `api_id` and `affiliate_id` (free registration at affiliate.dmm.com). Returns richer data than AVWiki: title, date, actors, genres, maker, label, series, cover, runtime, etc.

Ported from `pikpak-batch-renamer.user.js` `queryDMM()` and existing `pkg/scraper/dmm.go`.

**Files:**
- Create: `bangou/provider/dmm.go`
- Create: `bangou/provider/dmm_test.go`

- [ ] **Step 1: Write failing test**

```go
// provider/dmm_test.go
package provider

import (
	"context"
	"testing"
)

func TestDMMScrape(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	// These must be set for the test to work.
	// Run with: go test ./provider/ -v -run TestDMM -args -dmm-api-id=xxx -dmm-affiliate-id=xxx
	apiID := "your-test-api-id"
	affiliateID := "your-test-affiliate-id"

	p := NewDMM(apiID, affiliateID)
	ctx := context.Background()

	meta, err := p.Scrape(ctx, "ACHJ-057")
	if err != nil {
		t.Fatalf("Scrape failed: %v", err)
	}
	if meta == nil {
		t.Fatal("expected metadata, got nil")
	}
	if meta.Title == "" {
		t.Error("expected non-empty title")
	}

	t.Logf("title=%s date=%s actors=%v maker=%s cover=%s",
		meta.Title, meta.Premiered, meta.Actors, meta.Maker, meta.CoverURL)
}

func TestDMMScrapeNotConfigured(t *testing.T) {
	p := NewDMM("", "")
	ctx := context.Background()

	meta, err := p.Scrape(ctx, "ACHJ-057")
	if err == nil || meta != nil {
		t.Error("expected error when not configured")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd bangou && go test ./provider/ -v -run TestDMM -short
```

Expected: FAIL — `NewDMM` not defined.

- [ ] **Step 3: Implement DMM provider**

```go
// provider/dmm.go
package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const dmmAPIURL = "https://api.dmm.com/affiliate/v3/ItemList"

// DMM uses the DMM Affiliate API v3 (FANZA).
// Register at https://affiliate.dmm.com/ to get api_id and affiliate_id.
type DMM struct {
	apiID       string
	affiliateID string
	client      *http.Client
}

func NewDMM(apiID, affiliateID string) *DMM {
	return &DMM{
		apiID:       apiID,
		affiliateID: affiliateID,
		client:      &http.Client{Timeout: 15 * time.Second},
	}
}

func (d *DMM) Name() string { return "dmm" }

func (d *DMM) Scrape(ctx context.Context, number string) (*MovieMetadata, error) {
	if d.apiID == "" || d.affiliateID == "" {
		return nil, fmt.Errorf("dmm: not configured (missing api_id or affiliate_id)")
	}

	// Build search keyword: "achj00057" format (series + padded number)
	label, num := splitNumber(number)
	if label == "" {
		return nil, fmt.Errorf("dmm: cannot parse number %s", number)
	}
	keyword := fmt.Sprintf("%s00%s", strings.ToLower(label), num)

	u, _ := url.Parse(dmmAPIURL)
	q := u.Query()
	q.Set("api_id", d.apiID)
	q.Set("affiliate_id", d.affiliateID)
	q.Set("site", "FANZA")
	q.Set("keyword", keyword)
	q.Set("output", "json")
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		return nil, err
	}

	resp, err := d.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("dmm: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("dmm: HTTP %d", resp.StatusCode)
	}

	var result dmmResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("dmm: decode failed: %w", err)
	}

	if result.Result.Status != 200 || len(result.Result.Items) == 0 {
		return nil, fmt.Errorf("dmm: no results for %s", number)
	}

	item := result.Result.Items[0]
	return d.toMetadata(number, &item), nil
}

func (d *DMM) toMetadata(number string, item *dmmItem) *MovieMetadata {
	meta := &MovieMetadata{
		Number: number,
		Title:  sanitizeTitle(item.Title),
	}

	// Date: "2025-01-15 10:00:00" → "2025-01-15"
	if item.Date != "" {
		meta.Premiered = strings.Split(item.Date, " ")[0]
		if len(meta.Premiered) >= 4 {
			meta.Year = meta.Premiered[:4]
		}
	}

	// Actors
	if item.ItemInfo.Actress != nil {
		for _, a := range item.ItemInfo.Actress {
			meta.Actors = append(meta.Actors, a.Name)
		}
	}

	// Genres
	if item.ItemInfo.Genre != nil {
		for _, g := range item.ItemInfo.Genre {
			meta.Genres = append(meta.Genres, g.Name)
		}
	}

	// Director
	if item.ItemInfo.Director != nil && len(item.ItemInfo.Director) > 0 {
		meta.Director = item.ItemInfo.Director[0].Name
	}

	// Maker
	if item.ItemInfo.Maker != nil && len(item.ItemInfo.Maker) > 0 {
		meta.Maker = item.ItemInfo.Maker[0].Name
	}

	// Label
	if item.ItemInfo.Label != nil && len(item.ItemInfo.Label) > 0 {
		meta.Label = item.ItemInfo.Label[0].Name
	}

	// Series
	if item.ItemInfo.Series != nil && len(item.ItemInfo.Series) > 0 {
		meta.Series = item.ItemInfo.Series[0].Name
	}

	// Cover: prefer large image
	if item.ImageURL.Large != "" {
		meta.CoverURL = item.ImageURL.Large
	} else if item.ImageURL.Small != "" {
		meta.CoverURL = item.ImageURL.Small
	}

	// Runtime
	if item.Runtime != "" {
		meta.Runtime = item.Runtime
	}

	return meta
}

// splitNumber: "ACHJ-057" → ("ACHJ", "057"), "300MAAN-783" → ("300MAAN", "783")
func splitNumber(number string) (string, string) {
	idx := strings.LastIndex(number, "-")
	if idx == -1 {
		return "", ""
	}
	return number[:idx], number[idx+1:]
}

func sanitizeTitle(s string) string {
	// Remove characters invalid in filenames
	replacer := strings.NewReplacer("/", "_", ":", "_", "*", "_", "?", "_", "\"", "_", "<", "_", ">", "_", "|", "_")
	return replacer.Replace(s)
}

// DMM API response types

type dmmResponse struct {
	Result dmmResult `json:"result"`
}

type dmmResult struct {
	Status     int       `json:"status"`
	TotalCount int       `json:"total_count"`
	Items      []dmmItem `json:"items"`
}

type dmmItem struct {
	Title    string      `json:"title"`
	Date     string      `json:"date"`
	Runtime  string      `json:"runtime"`
	ImageURL dmmImageURL `json:"imageURL"`
	ItemInfo dmmItemInfo `json:"iteminfo"`
}

type dmmImageURL struct {
	Small string `json:"small"`
	Large string `json:"large"`
}

type dmmItemInfo struct {
	Genre    []dmmNameID `json:"genre"`
	Maker    []dmmNameID `json:"maker"`
	Actress  []dmmNameID `json:"actress"`
	Director []dmmNameID `json:"director"`
	Label    []dmmNameID `json:"label"`
	Series   []dmmNameID `json:"series"`
}

type dmmNameID struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}
```

- [ ] **Step 4: Run tests**

```bash
cd bangou && go test ./provider/ -v -run TestDMMScrapeNotConfigured
```

Expected: PASS (unconfigured returns error).

For full integration test (requires real credentials):

```bash
go test ./provider/ -v -run TestDMMScrape -args -dmm-api-id=YOUR_ID -dmm-affiliate-id=YOUR_AFF_ID
```

- [ ] **Step 5: Commit**

```bash
git add provider/dmm.go provider/dmm_test.go
git commit -m "feat: DMM Affiliate API provider"
```

---

## Task 4: NFO Generator

**Files:**
- Create: `bangou/nfo/nfo.go`
- Create: `bangou/nfo/nfo_test.go`

- [ ] **Step 1: Write failing test**

```go
// nfo/nfo_test.go
package nfo

import (
	"strings"
	"testing"

	"github.com/zeroAlcBeer/bangou/provider"
)

func TestGenerate(t *testing.T) {
	meta := &provider.MovieMetadata{
		Number:    "ACHJ-057",
		Title:     "Test Movie Title",
		Director:  "Test Director",
		Maker:     "Test Studio",
		Label:     "Test Label",
		Actors:    []string{"Actor A", "Actor B"},
		Genres:    []string{"Genre1", "Genre2"},
		Premiered: "2025-01-15",
		Year:      "2025",
	}

	xml, err := Generate(meta)
	if err != nil {
		t.Fatal(err)
	}

	s := string(xml)
	if !strings.Contains(s, "<title>ACHJ-057 Test Movie Title</title>") {
		t.Error("title not found in NFO")
	}
	if !strings.Contains(s, "<name>Actor A</name>") {
		t.Error("actor not found in NFO")
	}
	if !strings.Contains(s, "<genre>Genre1</genre>") {
		t.Error("genre not found in NFO")
	}
	if !strings.Contains(s, "<studio>Test Studio</studio>") {
		t.Error("studio not found in NFO")
	}
	if !strings.Contains(s, "<?xml") {
		t.Error("XML header missing")
	}

	t.Logf("NFO:\n%s", s)
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd bangou && go test ./nfo/ -v
```

- [ ] **Step 3: Implement NFO generator**

```go
// nfo/nfo.go
package nfo

import (
	"encoding/xml"
	"fmt"
	"os"

	"github.com/zeroAlcBeer/bangou/provider"
)

type movie struct {
	XMLName   xml.Name `xml:"movie"`
	Plot      string   `xml:"plot"`
	Title     string   `xml:"title"`
	Director  string   `xml:"director,omitempty"`
	Year      string   `xml:"year,omitempty"`
	Premiered string   `xml:"premiered,omitempty"`
	Runtime   string   `xml:"runtime,omitempty"`
	Genre     []string `xml:"genre"`
	Studio    string   `xml:"studio,omitempty"`
	Tag       []string `xml:"tag"`
	Actor     []actor  `xml:"actor"`
	Label     string   `xml:"label,omitempty"`
	Num       string   `xml:"num"`
	Cover     string   `xml:"cover,omitempty"`
}

type actor struct {
	Name string `xml:"name"`
}

// Generate creates Emby-compatible NFO XML from metadata.
func Generate(meta *provider.MovieMetadata) ([]byte, error) {
	if meta == nil {
		return nil, fmt.Errorf("nil metadata")
	}

	var actors []actor
	for _, a := range meta.Actors {
		actors = append(actors, actor{Name: a})
	}

	// Tags = genres + maker + label + series + director (Emby convention)
	tags := append([]string{}, meta.Genres...)
	for _, t := range []string{meta.Maker, meta.Label, meta.Series, meta.Director} {
		if t != "" {
			tags = append(tags, t)
		}
	}

	m := &movie{
		Plot:      meta.Plot,
		Title:     fmt.Sprintf("%s %s", meta.Number, meta.Title),
		Director:  meta.Director,
		Year:      meta.Year,
		Premiered: meta.Premiered,
		Runtime:   meta.Runtime,
		Genre:     meta.Genres,
		Studio:    meta.Maker,
		Tag:       tags,
		Actor:     actors,
		Label:     meta.Label,
		Num:       meta.Number,
		Cover:     meta.CoverURL,
	}

	x, err := xml.MarshalIndent(m, "", "  ")
	if err != nil {
		return nil, err
	}
	return []byte(xml.Header + string(x)), nil
}

// Save writes NFO XML to a file.
func Save(meta *provider.MovieMetadata, path string) error {
	data, err := Generate(meta)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
```

- [ ] **Step 4: Run test**

```bash
cd bangou && go test ./nfo/ -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add nfo/
git commit -m "feat: Emby-compatible NFO generator from metadata"
```

---

## Task 5: Cover Downloader

**Files:**
- Modify: `bangou/provider/provider.go` (add download helper)

- [ ] **Step 1: Add cover download function**

```go
// Add to provider/provider.go

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

var httpClient = &http.Client{Timeout: 30 * time.Second}

// DownloadCover downloads a cover image to the specified directory.
// Returns the local file path, or empty string on failure.
func DownloadCover(ctx context.Context, coverURL, outputDir, number string) string {
	if coverURL == "" {
		return ""
	}

	// Determine extension from URL
	ext := filepath.Ext(coverURL)
	if ext == "" || len(ext) > 5 {
		ext = ".jpg"
	}

	localPath := filepath.Join(outputDir, number+ext)

	req, err := http.NewRequestWithContext(ctx, "GET", coverURL, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := httpClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return ""
	}

	f, err := os.Create(localPath)
	if err != nil {
		return ""
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		os.Remove(localPath)
		return ""
	}

	return localPath
}
```

- [ ] **Step 2: Commit**

```bash
git add provider/
git commit -m "feat: cover image downloader"
```

---

## Task 6: Wire Scraping into Pipeline

Hook scraping into the existing `handleEvent` flow and the executor.

**Files:**
- Modify: `bangou/main.go`
- Modify: `bangou/executor/executor.go`

- [ ] **Step 1: Add auto-scrape to handleEvent**

In `main.go`, after the existing `EnsureGroup` call:

```go
// After: group, err := db.EnsureGroup(ctx, parsed.Number)
// Add:

// Auto-scrape metadata (non-blocking)
go func(number string) {
	// Skip if already have metadata
	if existing, _ := db.GetMetadata(context.Background(), number); existing != nil {
		return
	}

	providers := []provider.Provider{
		provider.NewAVWiki(),
		// future: provider.NewMetaTube(apiURL, token),
	}

	meta := provider.Chain(context.Background(), providers, number)
	if meta == nil {
		log.Printf("scrape: no metadata found for %s", number)
		return
	}

	// Save to DB
	db.UpsertMetadata(context.Background(), &store.Metadata{
		Number:    meta.Number,
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
	log.Printf("scrape: %s → %s (%s)", number, meta.Title, meta.Provider)
}(parsed.Number)
```

- [ ] **Step 2: Generate NFO + download cover on executor link/merge**

In `executor/executor.go`, add to both `doLink` and `doMerge` (after creating the output dir):

```go
// Add import
import (
	"github.com/zeroAlcBeer/bangou/nfo"
	"github.com/zeroAlcBeer/bangou/provider"
)

// Add method to Executor
func (e *Executor) writeMetadata(ctx context.Context, group *store.Group, outputDir string) {
	meta, err := e.store.GetMetadata(ctx, group.Number)
	if err != nil || meta == nil {
		return // no metadata, skip silently
	}

	// Convert store.Metadata → provider.MovieMetadata
	movieMeta := &provider.MovieMetadata{
		Number:    meta.Number,
		Title:     meta.Title,
		Plot:      meta.Plot,
		Director:  meta.Director,
		Maker:     meta.Maker,
		Label:     meta.Label,
		Series:    meta.Series,
		Actors:    splitCSV(meta.Actors),
		Genres:    splitCSV(meta.Genres),
		CoverURL:  meta.CoverURL,
		Premiered: meta.Premiered,
		Year:      meta.Year,
		Runtime:   meta.Runtime,
	}

	// Write NFO
	nfoPath := filepath.Join(outputDir, group.Number+".nfo")
	if err := nfo.Save(movieMeta, nfoPath); err != nil {
		log.Printf("warn: failed to write NFO for %s: %v", group.Number, err)
	}

	// Download cover
	provider.DownloadCover(ctx, meta.CoverURL, outputDir, group.Number)
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}
```

Call `writeMetadata` in `doLink`:

```go
func (e *Executor) doLink(ctx context.Context, group *store.Group, files []store.SourceFile) error {
	src := files[0]
	outDir := filepath.Join(e.outputDir, group.Number)
	os.MkdirAll(outDir, 0755)

	linkPath, err := LinkFile(src.Path, e.outputDir, group.Number, e.linkType)
	if err != nil {
		return err
	}

	// Write NFO + cover
	e.writeMetadata(ctx, group, outDir)

	// ... rest unchanged
}
```

Same for `doMerge` — call `e.writeMetadata(ctx, group, outDir)` after merge succeeds.

- [ ] **Step 3: Commit**

```bash
git add main.go executor/
git commit -m "feat: auto-scrape on parse, NFO + cover on link/merge"
```

---

## Task 7: Provider Settings in UI

Add provider configuration to Settings page: DMM API credentials, provider enable/disable, drag-to-reorder priority.

**Files:**
- Modify: `bangou/web/templates/settings.html`
- Modify: `bangou/web/handlers.go`

- [ ] **Step 1: Update settings template with provider section**

Add to `web/templates/settings.html`, after the aria2 section:

```html
    <div class="card">
        <h3 style="margin-bottom:12px">Metadata Providers</h3>
        <p style="color:#666; font-size:12px; margin-bottom:12px">
            Drag to reorder priority. First match wins.
        </p>

        <div id="provider-list" style="display:flex; flex-direction:column; gap:8px;">

            {{range .Providers}}
            <div class="provider-row" draggable="true"
                 style="display:flex; align-items:center; gap:12px; padding:10px; background:#0a0a1a; border-radius:4px; cursor:grab;"
                 data-name="{{.Name}}">
                <span style="color:#666">☰</span>
                <input type="checkbox" name="provider_enabled_{{.Name}}" {{if .Enabled}}checked{{end}}>
                <strong>{{.DisplayName}}</strong>
                <span style="color:#666; font-size:12px; flex:1">{{.Description}}</span>
                {{if .NeedsConfig}}
                    {{if .Configured}}
                        <span style="color:#1b4332; font-size:12px">✓ configured</span>
                    {{else}}
                        <span style="color:#e94560; font-size:12px">⚠ needs config</span>
                    {{end}}
                {{else}}
                    <span style="color:#444; font-size:12px">no config needed</span>
                {{end}}
            </div>
            {{end}}

        </div>

        <input type="hidden" name="provider_order" id="provider-order" value="{{.ProviderOrder}}">

        <script>
        // Minimal drag-to-reorder (no library needed)
        const list = document.getElementById('provider-list');
        let dragItem = null;
        list.addEventListener('dragstart', e => { dragItem = e.target.closest('.provider-row'); });
        list.addEventListener('dragover', e => { e.preventDefault(); });
        list.addEventListener('drop', e => {
            e.preventDefault();
            const target = e.target.closest('.provider-row');
            if (target && dragItem !== target) {
                list.insertBefore(dragItem, target);
            }
            // Update hidden field with new order
            const order = [...list.querySelectorAll('.provider-row')].map(r => r.dataset.name).join(',');
            document.getElementById('provider-order').value = order;
        });
        </script>
    </div>

    <div class="card">
        <h3 style="margin-bottom:12px">DMM API</h3>
        <p style="color:#666; font-size:12px; margin-bottom:12px">
            Register at <a href="https://affiliate.dmm.com/" style="color:#533483">affiliate.dmm.com</a> to get credentials (free).
        </p>
        <label>API ID</label>
        <input type="text" name="dmm_api_id" value="{{.dmm_api_id}}" style="width:100%; padding:8px; margin:8px 0; background:#0a0a1a; color:#fff; border:1px solid #333; border-radius:4px;">
        <label>Affiliate ID</label>
        <input type="text" name="dmm_affiliate_id" value="{{.dmm_affiliate_id}}" style="width:100%; padding:8px; margin:8px 0; background:#0a0a1a; color:#fff; border:1px solid #333; border-radius:4px;">
    </div>
```

- [ ] **Step 2: Define ProviderInfo view model and update Settings handler**

```go
// Add to web/handlers.go

type ProviderInfo struct {
	Name        string
	DisplayName string
	Description string
	Enabled     bool
	NeedsConfig bool
	Configured  bool
}

func (h *Handlers) Settings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	settings, _ := h.store.GetAllSettings(ctx)

	// Auto-detect mkvmerge
	mkvPath, err := exec.LookPath("mkvmerge")
	if err == nil {
		settings["mkvmerge_available"] = "true"
		settings["mkvmerge_path"] = mkvPath
	}

	// Auto-detect link type
	settings["link_type"] = detectLinkType(settings["input_dir"], settings["output_dir"])

	// Build provider info
	order := settings["provider_order"]
	if order == "" {
		order = "avwiki,dmm" // default order
	}

	dmmConfigured := settings["dmm_api_id"] != "" && settings["dmm_affiliate_id"] != ""
	enabledMap := map[string]bool{
		"avwiki": settings["provider_enabled_avwiki"] != "false", // default enabled
		"dmm":    settings["provider_enabled_dmm"] != "false",
	}

	providerDefs := map[string]ProviderInfo{
		"avwiki": {Name: "avwiki", DisplayName: "AV-Wiki", Description: "av-wiki.net — HTML scraping, no config needed", NeedsConfig: false, Configured: true},
		"dmm":    {Name: "dmm", DisplayName: "DMM API", Description: "FANZA Affiliate API — richer data (actors, genres, cover, runtime)", NeedsConfig: true, Configured: dmmConfigured},
	}

	var providers []ProviderInfo
	for _, name := range strings.Split(order, ",") {
		name = strings.TrimSpace(name)
		if pi, ok := providerDefs[name]; ok {
			pi.Enabled = enabledMap[name]
			providers = append(providers, pi)
		}
	}

	settings["Providers"] = ""      // not used directly, passed separately
	settings["ProviderOrder"] = order

	data := map[string]interface{}{
		"Settings":      settings,
		"Providers":     providers,
		"ProviderOrder": order,
	}
	// Flatten settings into data for template access
	for k, v := range settings {
		data[k] = v
	}

	renderTemplate(w, "layout.html", data)
}
```

- [ ] **Step 3: Update SaveSettings to handle provider settings**

```go
func (h *Handlers) SaveSettings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	r.ParseForm()

	// Standard settings
	keys := []string{"input_dir", "output_dir", "aria2_rpc_url", "aria2_token",
		"dmm_api_id", "dmm_affiliate_id", "provider_order"}
	for _, k := range keys {
		if v := r.FormValue(k); v != "" {
			h.store.SetSetting(ctx, k, v)
		}
	}

	// Provider enable/disable
	for _, name := range []string{"avwiki", "dmm"} {
		key := "provider_enabled_" + name
		if r.FormValue(key) != "" {
			h.store.SetSetting(ctx, key, "true")
		} else {
			h.store.SetSetting(ctx, key, "false")
		}
	}

	http.Redirect(w, r, "/settings", http.StatusSeeOther)
}
```

- [ ] **Step 4: Update pipeline to build provider chain from settings**

In `main.go`, replace the hardcoded provider list with settings-driven construction:

```go
// Add to main.go or a new file provider/factory.go

func buildProviders(ctx context.Context, db store.Store) []provider.Provider {
	settings, _ := db.GetAllSettings(ctx)

	order := settings["provider_order"]
	if order == "" {
		order = "avwiki,dmm"
	}

	var providers []provider.Provider
	for _, name := range strings.Split(order, ",") {
		name = strings.TrimSpace(name)

		// Skip disabled providers
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
		// case "metatube":
		//     ...future
		}
	}

	return providers
}
```

Update `handleEvent` auto-scrape to use this:

```go
// Replace hardcoded provider list with:
providers := buildProviders(context.Background(), db)
meta := provider.Chain(context.Background(), providers, number)
```

- [ ] **Step 5: Commit**

```bash
git add web/ main.go
git commit -m "feat: provider settings UI with ordering and DMM API config"
```

---

## Task 8: Show Metadata in Web UI

Display scraped metadata in the group card for review before user action.

**Files:**
- Modify: `bangou/web/handlers.go`
- Modify: `bangou/web/templates/index.html`

- [ ] **Step 1: Add metadata to GroupView**

```go
// Add to GroupView struct in handlers.go
type GroupView struct {
	Group       store.Group
	Files       []FileView
	Tags        []string
	HasNewParts bool
	LinkAlive   bool
	Meta        *store.Metadata // NEW
}

// Update buildGroupView
func (h *Handlers) buildGroupView(ctx context.Context, g store.Group) GroupView {
	// ... existing code ...

	meta, _ := h.store.GetMetadata(ctx, g.Number)

	return GroupView{
		Group: g,
		Files: fileViews,
		Meta:  meta,
	}
}
```

- [ ] **Step 2: Update index.html to show metadata**

In the pending group card, after the parts list and before the action buttons:

```html
{{if .Meta}}
<div style="margin:10px 0; padding:10px; background:#0a0a1a; border-radius:4px; font-size:13px;">
    {{if .Meta.Title}}<div><strong>{{.Meta.Title}}</strong></div>{{end}}
    {{if .Meta.Premiered}}<div style="color:#aaa">{{.Meta.Premiered}}</div>{{end}}
    {{if .Meta.Actors}}<div style="color:#aaa">{{.Meta.Actors}}</div>{{end}}
    {{if .Meta.Maker}}<div style="color:#666">{{.Meta.Maker}} {{if .Meta.Label}}/ {{.Meta.Label}}{{end}}</div>{{end}}
    {{if .Meta.CoverURL}}<img src="{{.Meta.CoverURL}}" style="max-width:200px; margin-top:8px; border-radius:4px;">{{end}}
    <div style="color:#444; font-size:11px; margin-top:4px">via {{.Meta.Provider}}</div>
</div>
{{else}}
<div style="margin:10px 0; color:#666; font-size:13px;">metadata not available</div>
{{end}}
```

- [ ] **Step 3: Commit**

```bash
git add web/
git commit -m "feat: display scraped metadata in group card"
```

---

## Task 9: Future MetaTube Provider (Stub)

Leave a stub for future MetaTube integration. Not implemented in MVP.

**Files:**
- Create: `bangou/provider/metatube.go`

- [ ] **Step 1: Create stub**

```go
// provider/metatube.go
package provider

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// MetaTube connects to a MetaTube server API.
// API docs: https://github.com/metatube-community/metatube-sdk-go
//
// Required settings: api_url, api_token (stored in DB settings table)
//
// Endpoints used:
//   GET /v1/movies/search?q={number}   → search
//   GET /v1/movies/{provider}/{id}     → detail
//   GET /v1/images/primary/{provider}/{id} → cover image

type MetaTube struct {
	apiURL string
	token  string
	client *http.Client
}

func NewMetaTube(apiURL, token string) *MetaTube {
	return &MetaTube{
		apiURL: apiURL,
		token:  token,
		client: &http.Client{Timeout: 15 * time.Second},
	}
}

func (m *MetaTube) Name() string { return "metatube" }

func (m *MetaTube) Scrape(ctx context.Context, number string) (*MovieMetadata, error) {
	// TODO: implement when ready
	// 1. GET {apiURL}/v1/movies/search?q={number}  (Bearer token)
	// 2. Parse first result → provider + id
	// 3. GET {apiURL}/v1/movies/{provider}/{id}
	// 4. Map response to MovieMetadata
	return nil, fmt.Errorf("metatube: not implemented")
}
```

- [ ] **Step 2: Commit**

```bash
git add provider/metatube.go
git commit -m "feat: MetaTube provider stub for future integration"
```

---

## Summary

| Task | What | Depends On |
|------|------|-----------|
| 1 | Provider interface + metadata DB | existing store |
| 2 | AVWiki provider | Task 1 |
| 3 | DMM API provider | Task 1 |
| 4 | NFO generator | Task 1 |
| 5 | Cover downloader | Task 1 |
| 6 | Wire into pipeline + executor | Tasks 1-5 |
| 7 | Provider settings in UI (order, DMM config) | Tasks 2-3 |
| 8 | Show metadata in group card | Task 6 |
| 9 | MetaTube stub | Task 1 |

**Design principles:**
- Provider chain: try sources in configured order, first success wins.
- Provider order and enable/disable configurable in Settings UI (drag to reorder).
- DMM API needs credentials (free registration); AVWiki needs nothing.
- Scrape async after parse, failure = silent skip, never blocks user decisions.
- NFO + cover written at link/merge time, not scrape time.
- Metadata in DB = source of truth. Can re-scrape with different provider to overwrite.
- Future: add MetaTube as additional provider. Just implement the interface + add to settings UI.
