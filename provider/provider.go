package provider

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type MovieMetadata struct {
	Number       string
	Title        string
	Plot         string
	Director     string
	Maker        string
	Label        string
	Series       string
	Actors       []string
	Genres       []string
	CoverURL     string
	SampleImages []string // sample/preview image URLs
	Premiered    string
	Year         string
	Runtime      string // minutes
	Rating       string // e.g. "4.00"
	ReviewCount  int
	SampleMovieURL string // preview video URL (largest available)
	PageURL        string // source page URL
	ContentID      string // provider internal ID
	Provider     string
	RawJSON      []byte `json:"-"` // original provider response for debugging
}

type Predict struct {
	Number    string
	RawNumber string // original cleaned identifier, e.g. "13dsvr01801"
}

type Provider interface {
	Name() string
	Scrape(ctx context.Context, p Predict) (*MovieMetadata, error)
}

type ScrapeResult struct {
	Meta   *MovieMetadata
	Errors map[string]string
}

func Chain(ctx context.Context, providers []Provider, p Predict) *ScrapeResult {
	log.Printf("[scrape] start number=%s raw=%s", p.Number, p.RawNumber)
	out := &ScrapeResult{Errors: map[string]string{}}

	// Index providers by name for supplemental lookup
	byName := map[string]Provider{}
	for _, prov := range providers {
		byName[prov.Name()] = prov
	}

	for _, prov := range providers {
		meta, err := prov.Scrape(ctx, p)
		if err != nil {
			out.Errors[prov.Name()] = err.Error()
			continue
		}
		if meta != nil {
			meta.Provider = prov.Name()
			out.Meta = meta

			// If primary result is from DMM and avwiki is also enabled, supplement
			if prov.Name() == "dmm" {
				if aw, ok := byName["avwiki"]; ok {
					supplementFromAVWiki(ctx, aw, p, meta)
				}
			}
			return out
		}
		out.Errors[prov.Name()] = "not found"
	}
	return out
}

// supplementFromAVWiki queries avwiki to fill in missing actors and override premiered date.
func supplementFromAVWiki(ctx context.Context, aw Provider, p Predict, meta *MovieMetadata) {
	awMeta, err := aw.Scrape(ctx, p)
	if err != nil || awMeta == nil {
		log.Printf("[scrape] avwiki supplement for %s: skipped (%v)", p.Number, err)
		return
	}
	log.Printf("[scrape] avwiki supplement for %s: got actors=%v premiered=%s", p.Number, awMeta.Actors, awMeta.Premiered)

	// Always use avwiki's date if available
	if awMeta.Premiered != "" {
		meta.Premiered = awMeta.Premiered
		if len(awMeta.Premiered) >= 4 {
			meta.Year = awMeta.Premiered[:4]
		}
	}

	// Fill actors if DMM has none
	if len(meta.Actors) == 0 && len(awMeta.Actors) > 0 {
		meta.Actors = awMeta.Actors
	}
}

var coverHTTPClient = &http.Client{Timeout: 30 * time.Second}

// DownloadCover downloads an image with basic safety checks.
// Returns local file path on success, or empty string on failure.
func DownloadCover(ctx context.Context, coverURL, outputDir, number string) string {
	if coverURL == "" {
		return ""
	}
	if !(strings.HasPrefix(coverURL, "http://") || strings.HasPrefix(coverURL, "https://")) {
		return ""
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, coverURL, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("User-Agent", "bangou/1.0")

	log.Printf("[cover] GET %s", coverURL)
	resp, err := coverHTTPClient.Do(req)
	if err != nil {
		log.Printf("[cover] GET %s -> error: %v", coverURL, err)
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("[cover] GET %s -> HTTP %d", coverURL, resp.StatusCode)
		return ""
	}
	if resp.ContentLength > 20<<20 {
		return ""
	}

	ct := strings.ToLower(strings.TrimSpace(resp.Header.Get("Content-Type")))
	if !strings.HasPrefix(ct, "image/") {
		return ""
	}
	mediaType, _, _ := mime.ParseMediaType(ct)
	ext := ".jpg"
	switch mediaType {
	case "image/png":
		ext = ".png"
	case "image/webp":
		ext = ".webp"
	case "image/jpeg", "image/jpg":
		ext = ".jpg"
	case "image/gif":
		ext = ".gif"
	}

	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return ""
	}
	localPath := filepath.Join(outputDir, number+ext)
	f, err := os.Create(localPath)
	if err != nil {
		return ""
	}
	defer f.Close()

	if _, err := io.Copy(f, io.LimitReader(resp.Body, 20<<20)); err != nil {
		_ = os.Remove(localPath)
		return ""
	}
	return localPath
}

func sanitizeTitle(s string) string {
	replacer := strings.NewReplacer("/", "_", ":", "_", "*", "_", "?", "_", "\"", "_", "<", "_", ">", "_", "|", "_")
	return replacer.Replace(strings.TrimSpace(s))
}

func splitNumber(number string) (string, string, error) {
	n := strings.TrimSpace(number)
	idx := strings.LastIndex(n, "-")
	if idx <= 0 || idx >= len(n)-1 {
		return "", "", fmt.Errorf("invalid number: %s", number)
	}
	label := n[:idx]
	num := n[idx+1:]
	if label == "" || num == "" {
		return "", "", errors.New("empty label or num")
	}
	return label, num, nil
}
