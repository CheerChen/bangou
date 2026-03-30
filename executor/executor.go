package executor

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"syscall"

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

// ResolveLinkPath expands a pattern like "{Year}/{Actor}/{Number}" using metadata.
// Only {Year}, {Actor}, {Number} are supported. {Number} is always appended if missing.
func ResolveLinkPath(pattern, number string, meta *provider.MovieMetadata) string {
	if strings.TrimSpace(pattern) == "" {
		return number
	}
	year := "Unknown"
	actor := "Unknown"
	if meta != nil {
		if meta.Year != "" {
			year = meta.Year
		}
		if len(meta.Actors) > 0 && meta.Actors[0] != "" {
			actor = meta.Actors[0]
		}
	}
	r := strings.NewReplacer(
		"{Year}", sanitizePath(year),
		"{Actor}", sanitizePath(actor),
		"{Number}", sanitizePath(number),
	)
	result := r.Replace(pattern)
	// Ensure number is always the last segment
	if !strings.HasSuffix(result, number) {
		result = filepath.Join(result, number)
	}
	return filepath.Clean(result)
}

func sanitizePath(s string) string {
	r := strings.NewReplacer("/", "_", "\\", "_", ":", "_", "*", "_", "?", "_", "\"", "_", "<", "_", ">", "_", "|", "_")
	return r.Replace(strings.TrimSpace(s))
}

func (e *Executor) Link(ctx context.Context, number string, selectedPaths []string) error {
	log.Printf("[link] %s: start, %d files selected", number, len(selectedPaths))
	group := e.staging.GetGroup(number)
	if group == nil {
		return fmt.Errorf("group %s not found", number)
	}

	selected := filterItems(group.Items, selectedPaths)
	if len(selected) == 0 {
		return fmt.Errorf("no matching files for %s", number)
	}

	// Resolve link path pattern
	pattern, _ := e.store.GetSetting(ctx, "link_path_pattern")
	relPath := ResolveLinkPath(pattern, number, group.Scrape.Meta)
	outDir := filepath.Join(e.outputDir, relPath)
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", outDir, err)
	}

	linkType := detectLinkType(filepath.Dir(selected[0].File.Path), e.outputDir)
	multiPart := len(selected) > 1
	log.Printf("[link] %s: linkType=%s multiPart=%v outDir=%s", number, linkType, multiPart, outDir)

	for i, item := range selected {
		part := i + 1
		log.Printf("[link] %s: linking part %d: %s", number, part, item.File.Filename)
		linkPath, err := LinkFile(item.File.Path, outDir, number, linkType, multiPart, part)
		if err != nil {
			return fmt.Errorf("link part %d: %w", part, err)
		}
		if err := e.store.CreateOutput(ctx, &committed.Output{Number: number, LinkPath: linkPath, LinkType: linkType}); err != nil {
			return fmt.Errorf("store output: %w", err)
		}
		log.Printf("[link] %s: part %d -> %s", number, part, linkPath)
	}

	e.writeMetadata(ctx, number, outDir, group.Scrape.Meta)
	if group.Scrape.Meta != nil {
		e.commitMetadata(ctx, number, group.Scrape.Meta)
	}
	e.staging.RemoveGroup(number)
	log.Printf("[link] %s: done, %d files linked", number, len(selected))
	return nil
}

// Merge combines selected parts into a single mkv in the same input directory.
// Does not create output links or remove the group — user should Rescan and then Link.
func (e *Executor) Merge(ctx context.Context, number string, selectedPaths []string) error {
	log.Printf("[merge] %s: start, %d files selected", number, len(selectedPaths))
	group := e.staging.GetGroup(number)
	if group == nil {
		return fmt.Errorf("group %s not found", number)
	}

	selected := filterItems(group.Items, selectedPaths)
	if len(selected) < 2 {
		return fmt.Errorf("need 2+ files to merge %s", number)
	}

	// Output to same directory as source files
	inputDir := filepath.Dir(selected[0].File.Path)
	mergedPath := filepath.Join(inputDir, number+".mkv")

	parts := make([]string, 0, len(selected))
	for _, item := range selected {
		parts = append(parts, item.File.Path)
		log.Printf("[merge] %s: input part: %s (%.2f GB)", number, item.File.Filename, float64(item.File.Size)/(1024*1024*1024))
	}
	log.Printf("[merge] %s: running mkvmerge -> %s", number, mergedPath)
	if err := MergeFiles(parts, mergedPath, func(pct int) {
		log.Printf("[merge] %s: progress %d%%", number, pct)
		e.staging.SetTaskProgress(number, pct)
	}); err != nil {
		return fmt.Errorf("mkvmerge: %w", err)
	}
	log.Printf("[merge] %s: done, %d parts -> %s", number, len(parts), mergedPath)
	return nil
}

// Unlink removes link files, nfo, cover, and raw from the output directory (keeps the directory).
// Deletes DB output and metadata records.
func (e *Executor) Unlink(ctx context.Context, number string, outputID int64) error {
	log.Printf("[unlink] %s: start", number)

	outDir := filepath.Join(e.outputDir, number)
	entries, err := os.ReadDir(outDir)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read dir: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		path := filepath.Join(outDir, entry.Name())
		log.Printf("[unlink] %s: removing %s", number, entry.Name())
		if err := os.Remove(path); err != nil {
			log.Printf("[unlink] %s: warn: %v", number, err)
		}
	}

	if err := e.store.DeleteOutput(ctx, outputID); err != nil {
		return fmt.Errorf("delete output: %w", err)
	}
	log.Printf("[unlink] %s: done", number)
	return nil
}

func (e *Executor) writeMetadata(ctx context.Context, number, outDir string, meta *provider.MovieMetadata) {
	if meta == nil {
		return
	}
	nfoPath := filepath.Join(outDir, number+".nfo")
	if err := nfo.Save(meta, nfoPath); err != nil {
		log.Printf("warn: write nfo %s: %v", number, err)
	}
	_ = provider.DownloadCover(ctx, meta.CoverURL, outDir, number)

	// Write raw provider response for debugging
	if len(meta.RawJSON) > 0 {
		rawPath := filepath.Join(outDir, number+"-raw."+meta.Provider)
		if err := os.WriteFile(rawPath, meta.RawJSON, 0o644); err != nil {
			log.Printf("warn: write raw %s: %v", number, err)
		} else {
			log.Printf("[link] %s: raw response saved to %s", number, rawPath)
		}
	}
}

func (e *Executor) commitMetadata(ctx context.Context, number string, meta *provider.MovieMetadata) {
	_ = e.store.UpsertMetadata(ctx, &committed.Metadata{
		Number:       number,
		Title:        meta.Title,
		Plot:         meta.Plot,
		Director:     meta.Director,
		Maker:        meta.Maker,
		Label:        meta.Label,
		Series:       meta.Series,
		Actors:       strings.Join(meta.Actors, ","),
		Genres:       strings.Join(meta.Genres, ","),
		CoverURL:     meta.CoverURL,
		SampleImages: strings.Join(meta.SampleImages, ","),
		Premiered:    meta.Premiered,
		Year:         meta.Year,
		Runtime:      meta.Runtime,
		Rating:       meta.Rating,
		ReviewCount:  meta.ReviewCount,
		PageURL:      meta.PageURL,
		ContentID:    meta.ContentID,
		Provider:     meta.Provider,
	})
}

func filterItems(items []staging.StagedItem, paths []string) []staging.StagedItem {
	set := make(map[string]bool, len(paths))
	for _, p := range paths {
		set[p] = true
	}
	out := make([]staging.StagedItem, 0, len(paths))
	for _, item := range items {
		if set[item.File.Path] {
			out = append(out, item)
		}
	}
	return out
}

func detectLinkType(input, output string) string {
	if input == "" || output == "" {
		return "symlink"
	}
	var inStat, outStat syscall.Stat_t
	if syscall.Stat(input, &inStat) != nil || syscall.Stat(output, &outStat) != nil {
		return "symlink"
	}
	if inStat.Dev == outStat.Dev {
		return "hardlink"
	}
	return "symlink"
}
