package committed

import "time"

type Pipeline struct {
	ID               int64
	Name             string
	InputDir         string
	OutputDir        string
	PathPattern      string
	ArchiveDir       string
	EnableMerge      bool
	DownloadProvider string // "none" | "aria2"
	ScrapeProviders  string // comma-separated ordered list, e.g. "dmm,avwiki"
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type ProviderConfig struct {
	Provider string
	Config   string // JSON blob
}

type Output struct {
	ID         int64
	PipelineID int64
	Number     string
	SrcPath    string
	LinkPath   string
	LinkType   string
	FileSize   int64
	Resolution string
	VideoCodec string
	AudioCodec string
	Duration   string
	Bitrate    string
	Alive      bool
	CreatedAt  time.Time
	CheckedAt  time.Time
}

type MergedPart struct {
	Number   string
	Filename string
	Size     int64
	Part     int
}

type Metadata struct {
	ID           int64
	PipelineID   int64
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
	SampleImages string // comma-separated URLs
	Premiered    string
	Year         string
	Runtime      string
	Rating       string
	ReviewCount  int
	PageURL      string
	ContentID    string
	Provider     string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
