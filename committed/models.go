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
