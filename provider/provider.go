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

type providerResult struct {
	name string
	meta *MovieMetadata
	err  error
}

// ScrapeAll runs all providers concurrently.
// onFirst is called as soon as the first provider succeeds (may be nil).
// After all providers finish (or ctx expires), supplementary results are merged.
func ScrapeAll(ctx context.Context, providers []Provider, p Predict, onFirst func(*ScrapeResult)) *ScrapeResult {
	log.Printf("[scrape] start number=%s raw=%s", p.Number, p.RawNumber)

	ch := make(chan providerResult, len(providers))
	for _, prov := range providers {
		go func(prov Provider) {
			meta, err := prov.Scrape(ctx, p)
			ch <- providerResult{name: prov.Name(), meta: meta, err: err}
		}(prov)
	}

	out := &ScrapeResult{Errors: map[string]string{}}
	var supplements []*MovieMetadata
	firstDone := false

	for range len(providers) {
		r := <-ch
		if r.err != nil {
			out.Errors[r.name] = r.err.Error()
		} else if r.meta == nil {
			out.Errors[r.name] = "not found"
		} else {
			r.meta.Provider = r.name
			if !firstDone {
				out.Meta = r.meta
				firstDone = true
				if onFirst != nil {
					onFirst(&ScrapeResult{Meta: r.meta, Errors: copyErrors(out.Errors)})
				}
			} else {
				supplements = append(supplements, r.meta)
			}
		}
	}

	if out.Meta != nil && len(supplements) > 0 {
		for _, sup := range supplements {
			mergeMeta(out.Meta, sup)
		}
	}
	return out
}

// mergeMeta patches base with supplementary data.
// Rule 1: if base (dmm) has no actors, fill from supplement.
// Rule 2: if base is dmm and premiered differs, prefer non-dmm date.
func mergeMeta(base, sup *MovieMetadata) {
	// Rule 1: fill empty actors from any provider
	if len(base.Actors) == 0 && len(sup.Actors) > 0 {
		log.Printf("[scrape] supplement %s: fill actors from %s: %v", base.Number, sup.Provider, sup.Actors)
		base.Actors = sup.Actors
	}
	// Rule 2: dmm premiered is unreliable — prefer non-dmm date
	if sup.Premiered != "" && sup.Premiered != base.Premiered {
		if base.Provider == "dmm" {
			log.Printf("[scrape] supplement %s: override dmm premiered %s -> %s (from %s)", base.Number, base.Premiered, sup.Premiered, sup.Provider)
			base.Premiered = sup.Premiered
			if len(sup.Premiered) >= 4 {
				base.Year = sup.Premiered[:4]
			}
		} else if sup.Provider == "dmm" {
			// dmm is supplement, keep base's date
			log.Printf("[scrape] supplement %s: ignore dmm premiered %s, keep %s (from %s)", base.Number, sup.Premiered, base.Premiered, base.Provider)
		} else {
			// neither is dmm, prefer supplement
			log.Printf("[scrape] supplement %s: override premiered %s -> %s (from %s)", base.Number, base.Premiered, sup.Premiered, sup.Provider)
			base.Premiered = sup.Premiered
			if len(sup.Premiered) >= 4 {
				base.Year = sup.Premiered[:4]
			}
		}
	}
}

func copyErrors(src map[string]string) map[string]string {
	cp := make(map[string]string, len(src))
	for k, v := range src {
		cp[k] = v
	}
	return cp
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
