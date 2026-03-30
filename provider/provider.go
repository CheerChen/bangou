package provider

import (
	"context"
	"errors"
	"fmt"
	"io"
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
	PageURL      string // source page URL
	ContentID    string // provider internal ID
	Provider     string
	RawJSON      []byte `json:"-"` // original provider response for debugging
}

type Provider interface {
	Name() string
	Scrape(ctx context.Context, number string) (*MovieMetadata, error)
}

type ScrapeResult struct {
	Meta   *MovieMetadata
	Errors map[string]string
}

func Chain(ctx context.Context, providers []Provider, number string) *ScrapeResult {
	out := &ScrapeResult{Errors: map[string]string{}}
	for _, p := range providers {
		meta, err := p.Scrape(ctx, number)
		if err != nil {
			out.Errors[p.Name()] = err.Error()
			continue
		}
		if meta != nil {
			meta.Provider = p.Name()
			out.Meta = meta
			return out
		}
		out.Errors[p.Name()] = "not found"
	}
	return out
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

	resp, err := coverHTTPClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
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
