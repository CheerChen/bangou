package staging

import (
	"github.com/zeroAlcBeer/bangou/provider"
	"github.com/zeroAlcBeer/bangou/scanner"
)

type StagingFile struct {
	Path           string
	Filename       string
	Size           int64
	Ready          bool
	Media          *scanner.MediaInfo
	DownloadPct    int    // 0-100, from aria2
	DownloadSize   int64  // bytes completed
	DownloadStatus string // "active", "waiting", "paused", "complete", "removed", "error", ""
}

type ParsedFile struct {
	Number     string
	RawNumber  string
	Part       int
	Tags       []string
	SourceSite string
}

type StagedItem struct {
	File   StagingFile
	Parsed ParsedFile
}

type ScrapeResult struct {
	Meta   *provider.MovieMetadata
	Errors map[string]string
	Status string // "", "scraping", "success", "failed"
}

type StagingGroup struct {
	Number       string
	RawNumber    string
	Items        []StagedItem
	Scrape       ScrapeResult
	Task         string // "", "linking", "merging", "done", "error"
	TaskErr      string
	TaskProgress int // 0-100
}

type UnknownFile struct {
	StagingFile
}
