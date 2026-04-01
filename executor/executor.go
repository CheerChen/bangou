package executor

import (
	"context"
	"fmt"
	"io"
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

// LinkOptions carries per-pipeline config for a link operation.
type LinkOptions struct {
	PipelineID  int64
	PathPattern string
	ArchiveDir  string // empty = skip archive
}

func New(store committed.Store, stg *staging.Manager, outputDir string) *Executor {
	return &Executor{store: store, staging: stg, outputDir: outputDir}
}

// ResolveLinkPath expands a pattern like "{Year}/{Actor}/{Number}" using metadata.
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
	if !strings.HasSuffix(result, number) {
		result = filepath.Join(result, number)
	}
	return filepath.Clean(result)
}

func sanitizePath(s string) string {
	r := strings.NewReplacer("/", "_", "\\", "_", ":", "_", "*", "_", "?", "_", "\"", "_", "<", "_", ">", "_", "|", "_")
	return r.Replace(strings.TrimSpace(s))
}

func (e *Executor) Link(ctx context.Context, number string, selectedPaths []string, opts ...LinkOptions) error {
	log.Printf("[link] %s: start, %d files selected", number, len(selectedPaths))
	group := e.staging.GetGroup(number)
	if group == nil {
		return fmt.Errorf("group %s not found", number)
	}

	selected := filterItems(group.Items, selectedPaths)
	if len(selected) == 0 {
		return fmt.Errorf("no matching files for %s", number)
	}

	// Resolve options
	var opt LinkOptions
	if len(opts) > 0 {
		opt = opts[0]
	}
	pattern := opt.PathPattern
	if pattern == "" {
		pattern, _ = e.store.GetSetting(ctx, "link_path_pattern")
	}

	// Archive: move source files to archive dir before linking
	if opt.ArchiveDir != "" {
		for i, item := range selected {
			archived, err := archiveFile(item.File.Path, opt.ArchiveDir)
			if err != nil {
				return fmt.Errorf("archive %s: %w", item.File.Filename, err)
			}
			log.Printf("[link] %s: archived %s -> %s", number, item.File.Path, archived)
			// Update the path so linking uses the archived location
			selected[i].File.Path = archived
		}
	}

	relPath := ResolveLinkPath(pattern, number, group.Scrape.Meta)
	outDir := filepath.Join(e.outputDir, relPath)
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", outDir, err)
	}

	srcDir := filepath.Dir(selected[0].File.Path)
	linkType := detectLinkType(srcDir, e.outputDir)
	multiPart := len(selected) > 1
	log.Printf("[link] %s: linkType=%s multiPart=%v outDir=%s", number, linkType, multiPart, outDir)

	for i, item := range selected {
		part := i + 1
		log.Printf("[link] %s: linking part %d: %s", number, part, item.File.Filename)
		result, err := LinkFile(item.File.Path, outDir, number, linkType, multiPart, part)
		if err != nil {
			return fmt.Errorf("link part %d: %w", part, err)
		}
		output := &committed.Output{
			PipelineID: opt.PipelineID,
			Number:     number,
			SrcPath:    item.File.Path,
			LinkPath:   result.LinkPath,
			LinkType:   result.LinkType,
			FileSize:   item.File.Size,
		}
		if item.File.Media != nil {
			output.Resolution = item.File.Media.Resolution()
			output.VideoCodec = item.File.Media.VideoCodec
			output.AudioCodec = item.File.Media.AudioCodec
			output.Duration = item.File.Media.DurationText()
			output.Bitrate = item.File.Media.BitrateText()
		}
		if err := e.store.CreateOutput(ctx, output); err != nil {
			return fmt.Errorf("store output: %w", err)
		}
		log.Printf("[link] %s: part %d %s %s -> %s", number, part, result.LinkType, item.File.Path, result.LinkPath)
	}

	e.writeMetadata(ctx, number, outDir, group.Scrape.Meta, opt.PipelineID)
	e.staging.RemoveGroup(number)
	log.Printf("[link] %s: done, %d files linked", number, len(selected))
	return nil
}

// Merge combines selected parts into a single mkv in the same input directory.
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

	inputDir := filepath.Dir(selected[0].File.Path)
	mergedPath := filepath.Join(inputDir, number+".mkv")

	parts := make([]string, 0, len(selected))
	var totalSize int64
	for _, item := range selected {
		parts = append(parts, item.File.Path)
		totalSize += item.File.Size
		log.Printf("[merge] %s: input part: %s (%.2f GB)", number, item.File.Filename, float64(item.File.Size)/(1024*1024*1024))
	}
	log.Printf("[merge] %s: running mkvmerge -> %s (total %.2f GB)", number, mergedPath, float64(totalSize)/(1024*1024*1024))
	if err := MergeFiles(parts, mergedPath, totalSize, func(pct int) {
		log.Printf("[merge] %s: progress %d%%", number, pct)
		e.staging.SetTaskProgress(number, pct)
	}); err != nil {
		return fmt.Errorf("mkvmerge: %w", err)
	}
	log.Printf("[merge] %s: done, %d parts -> %s", number, len(parts), mergedPath)
	return nil
}

// Unlink removes link files, nfo, cover, and raw from the output directory.
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

func (e *Executor) writeMetadata(ctx context.Context, number, outDir string, meta *provider.MovieMetadata, pipelineID int64) {
	if meta == nil {
		return
	}
	nfoPath := filepath.Join(outDir, number+".nfo")
	if err := nfo.Save(meta, nfoPath); err != nil {
		log.Printf("warn: write nfo %s: %v", number, err)
	}
	_ = provider.DownloadCover(ctx, meta.CoverURL, outDir, number)

	if len(meta.RawJSON) > 0 {
		rawPath := filepath.Join(outDir, number+"-raw."+meta.Provider)
		if err := os.WriteFile(rawPath, meta.RawJSON, 0o644); err != nil {
			log.Printf("warn: write raw %s: %v", number, err)
		}
	}

	e.commitMetadata(ctx, number, meta, pipelineID)
}

func (e *Executor) commitMetadata(ctx context.Context, number string, meta *provider.MovieMetadata, pipelineID int64) {
	_ = e.store.UpsertMetadata(ctx, &committed.Metadata{
		PipelineID:   pipelineID,
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

// archiveFile moves a file from srcPath to archiveDir, preserving the filename.
// Uses os.Rename for same-device moves, falls back to copy+delete for cross-device.
func archiveFile(srcPath, archiveDir string) (string, error) {
	if err := os.MkdirAll(archiveDir, 0o755); err != nil {
		return "", fmt.Errorf("mkdir archive: %w", err)
	}
	dst := filepath.Join(archiveDir, filepath.Base(srcPath))

	// Try rename first (instant if same device)
	if err := os.Rename(srcPath, dst); err == nil {
		return dst, nil
	}

	// Cross-device: copy then delete
	log.Printf("[archive] cross-device move: %s -> %s", srcPath, dst)
	src, err := os.Open(srcPath)
	if err != nil {
		return "", err
	}
	defer src.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return "", err
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, src); err != nil {
		os.Remove(dst)
		return "", fmt.Errorf("copy: %w", err)
	}
	if err := dstFile.Close(); err != nil {
		os.Remove(dst)
		return "", err
	}
	src.Close()
	if err := os.Remove(srcPath); err != nil {
		log.Printf("[archive] warn: remove source after copy: %v", err)
	}
	return dst, nil
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
