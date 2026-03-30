package store

import "time"

type SourceFile struct {
	ID        int64
	Path      string
	Filename  string
	Size      int64
	Ready     bool
	Ignored   bool
	CreatedAt time.Time
}

type ParsedInfo struct {
	ID         int64
	FileID     int64
	Number     string
	Part       int
	SourceSite string
	Tags       string
	Manual     bool
	UpdatedAt  time.Time
}

type Group struct {
	ID        int64
	Number    string
	Status    string // pending, merging, merge_error, done, ignored
	Action    string // link, merge, ignore
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Output struct {
	ID        int64
	GroupID   int64
	LinkPath  string
	LinkType  string
	Alive     bool
	CheckedAt time.Time
}

type MergedPart struct {
	ID       int64
	GroupID  int64
	Filename string
	Size     int64
	Part     int
}

type Metadata struct {
	ID           int64
	Number       string
	Title        string
	Plot         string
	Director     string
	Maker        string
	Label        string
	Series       string
	Actors       string
	Genres       string
	CoverURL     string
	CoverLocal   string
	Premiered    string
	Year         string
	Runtime      string
	Provider     string
	ScrapeStatus string
	ScrapeErrors string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
