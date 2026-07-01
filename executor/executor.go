package executor

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/zeroAlcBeer/bangou/committed"
	"github.com/zeroAlcBeer/bangou/nfo"
	"github.com/zeroAlcBeer/bangou/provider"
	"github.com/zeroAlcBeer/bangou/staging"
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
	if err := validateSingleExtensionSelection(selected); err != nil {
		return fmt.Errorf("link selection invalid for %s: %w", number, err)
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
			selected[i].File.Path = archived
		}
	}

	relPath := ResolveLinkPath(pattern, number, group.Scrape.Meta)
	outDir := filepath.Join(e.outputDir, relPath)
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", outDir, err)
	}

	// Create the Bangou aggregate root
	bangouID, err := e.store.CreateBangou(ctx, &committed.Bangou{
		PipelineID: opt.PipelineID,
		Number:     number,
		OutDir:     outDir,
	})
	if err != nil {
		return fmt.Errorf("create bangou: %w", err)
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
		bf := &committed.BangouFile{
			BangouID: bangouID,
			SrcPath:  item.File.Path,
			LinkPath: result.LinkPath,
			LinkType: result.LinkType,
			FileSize: item.File.Size,
		}
		if item.File.Media != nil {
			bf.Resolution = item.File.Media.Resolution()
			bf.VideoCodec = item.File.Media.VideoCodec
			bf.AudioCodec = item.File.Media.AudioCodec
			bf.Duration = item.File.Media.DurationText()
			bf.Bitrate = item.File.Media.BitrateText()
		}
		if err := e.store.CreateBangouFile(ctx, bf); err != nil {
			return fmt.Errorf("store bangou file: %w", err)
		}
		log.Printf("[link] %s: part %d %s %s -> %s", number, part, result.LinkType, item.File.Path, result.LinkPath)
	}

	e.writeMetadata(ctx, bangouID, number, outDir, group.Scrape.Meta)
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
	if hasMixedMKVAndMP4(group.Items) {
		return fmt.Errorf("merge is disabled for mixed mkv/mp4 groups")
	}

	inputDir := filepath.Dir(selected[0].File.Path)
	mergeTag, err := buildMergeSourceTag(selected)
	if err != nil {
		return fmt.Errorf("invalid merge selection for %s: %w", number, err)
	}
	mergedPath := filepath.Join(inputDir, number+"_"+mergeTag+".mkv")

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

// Unlink removes a linked file. When it's the last file under its Bangou,
// also removes metadata artifacts (nfo, cover, raw) and the Bangou record.
func (e *Executor) Unlink(ctx context.Context, file *committed.BangouFile) error {
	if file == nil {
		return fmt.Errorf("nil bangou file")
	}

	bangou, err := e.store.GetBangou(ctx, file.BangouID)
	if err != nil {
		return fmt.Errorf("get bangou: %w", err)
	}
	if bangou == nil {
		return fmt.Errorf("bangou %d not found", file.BangouID)
	}
	log.Printf("[unlink] %s: start (file %d)", bangou.Number, file.ID)

	// 1. Remove the link file
	linkPath := strings.TrimSpace(file.LinkPath)
	if linkPath == "" {
		return fmt.Errorf("empty link path")
	}
	if err := os.Remove(linkPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove link file: %w", err)
	}
	log.Printf("[unlink] %s: removed %s", bangou.Number, filepath.Base(linkPath))

	// 2. Delete the file record
	if err := e.store.DeleteBangouFile(ctx, file.ID); err != nil {
		return fmt.Errorf("delete bangou file: %w", err)
	}

	// 3. If this was the last file, clean up the entire Bangou
	remaining, err := e.store.CountBangouFiles(ctx, bangou.ID)
	if err != nil {
		return fmt.Errorf("count remaining files: %w", err)
	}
	if remaining == 0 {
		log.Printf("[unlink] %s: last file removed, cleaning up bangou", bangou.Number)
		e.cleanupBangouArtifacts(bangou)
		if err := e.store.DeleteBangou(ctx, bangou.ID); err != nil {
			return fmt.Errorf("delete bangou: %w", err)
		}
	}

	log.Printf("[unlink] %s: done", bangou.Number)
	return nil
}

// RestoreLink recreates a missing linked file from its stored source path.
func (e *Executor) RestoreLink(ctx context.Context, file *committed.BangouFile) error {
	if file == nil {
		return fmt.Errorf("nil bangou file")
	}
	if strings.TrimSpace(file.SrcPath) == "" {
		return fmt.Errorf("source path is unknown")
	}
	if strings.TrimSpace(file.LinkPath) == "" {
		return fmt.Errorf("link path is empty")
	}
	if _, err := os.Stat(file.SrcPath); err != nil {
		return fmt.Errorf("source missing: %w", err)
	}

	outDir := filepath.Dir(file.LinkPath)
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", outDir, err)
	}
	_ = os.Remove(file.LinkPath)

	linkType := detectLinkType(filepath.Dir(file.SrcPath), outDir)
	actualType := linkType
	switch linkType {
	case "hardlink":
		if err := os.Link(file.SrcPath, file.LinkPath); err != nil {
			log.Printf("[restore] hardlink failed, falling back to symlink: %v", err)
			actualType = "symlink"
			if err := os.Symlink(file.SrcPath, file.LinkPath); err != nil {
				return fmt.Errorf("symlink fallback: %w", err)
			}
		}
	case "symlink":
		if err := os.Symlink(file.SrcPath, file.LinkPath); err != nil {
			return fmt.Errorf("symlink: %w", err)
		}
	default:
		return fmt.Errorf("unknown link type: %s", linkType)
	}

	if actualType != file.LinkType {
		if err := e.store.SetBangouFileLinkType(ctx, file.ID, actualType); err != nil {
			return fmt.Errorf("update link type: %w", err)
		}
	}
	if err := e.store.SetBangouFileAlive(ctx, file.ID, true); err != nil {
		return fmt.Errorf("mark alive: %w", err)
	}
	log.Printf("[restore] file %d: %s -> %s", file.ID, file.SrcPath, file.LinkPath)
	return nil
}

// cleanupBangouArtifacts removes nfo, cover, raw files tracked by the Bangou.
func (e *Executor) cleanupBangouArtifacts(b *committed.Bangou) {
	for _, path := range []string{b.NFOPath, b.CoverPath, b.RawPath} {
		if path == "" {
			continue
		}
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			log.Printf("[unlink] %s: warn: remove %s: %v", b.Number, filepath.Base(path), err)
		} else if err == nil {
			log.Printf("[unlink] %s: removed %s", b.Number, filepath.Base(path))
		}
	}
	// Try to remove the output directory if empty
	if b.OutDir != "" {
		_ = os.Remove(b.OutDir) // only succeeds if empty
	}
}

func (e *Executor) writeMetadata(ctx context.Context, bangouID int64, number, outDir string, meta *provider.MovieMetadata) {
	if meta == nil {
		return
	}

	var nfoPath, coverPath, rawPath string

	nfoPath = filepath.Join(outDir, number+".nfo")
	if err := nfo.Save(meta, nfoPath); err != nil {
		log.Printf("warn: write nfo %s: %v", number, err)
		nfoPath = ""
	}

	coverPath = provider.DownloadCover(ctx, meta.CoverURL, outDir, number)

	if len(meta.RawJSON) > 0 {
		rawPath = filepath.Join(outDir, number+"-raw."+meta.Provider)
		if err := os.WriteFile(rawPath, meta.RawJSON, 0o644); err != nil {
			log.Printf("warn: write raw %s: %v", number, err)
			rawPath = ""
		}
	}

	// Record artifact paths on Bangou
	if err := e.store.UpdateBangouPaths(ctx, bangouID, nfoPath, coverPath, rawPath); err != nil {
		log.Printf("warn: update bangou paths %s: %v", number, err)
	}

	e.commitMetadata(ctx, bangouID, number, meta)
}

func (e *Executor) commitMetadata(ctx context.Context, bangouID int64, number string, meta *provider.MovieMetadata) {
	_ = e.store.UpsertMetadata(ctx, &committed.Metadata{
		BangouID:       bangouID,
		Number:         number,
		Title:          meta.Title,
		Plot:           meta.Plot,
		Director:       meta.Director,
		Maker:          meta.Maker,
		Label:          meta.Label,
		Series:         meta.Series,
		Actors:         strings.Join(meta.Actors, ","),
		Genres:         strings.Join(meta.Genres, ","),
		CoverURL:       meta.CoverURL,
		SampleImages:   strings.Join(meta.SampleImages, ","),
		Premiered:      meta.Premiered,
		Year:           meta.Year,
		Runtime:        meta.Runtime,
		Rating:         meta.Rating,
		ReviewCount:    meta.ReviewCount,
		SampleMovieURL: meta.SampleMovieURL,
		PageURL:        meta.PageURL,
		ContentID:      meta.ContentID,
		Provider:       meta.Provider,
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

func buildMergeSourceTag(selected []staging.StagedItem) (string, error) {
	if len(selected) == 0 {
		return "", fmt.Errorf("no selected files")
	}

	withPart := 0
	for _, item := range selected {
		if item.Parsed.Part > 0 {
			withPart++
		}
	}
	if withPart > 0 && withPart < len(selected) {
		return "", fmt.Errorf("part numbers must be either all present or all absent")
	}

	var b strings.Builder
	b.WriteString("m")

	if withPart == 0 {
		for i := 1; i <= len(selected); i++ {
			b.WriteString(strconv.Itoa(i))
		}
		return b.String(), nil
	}

	seen := make(map[int]struct{}, len(selected))
	for _, item := range selected {
		p := item.Parsed.Part
		if _, ok := seen[p]; ok {
			return "", fmt.Errorf("duplicate part number: %d", p)
		}
		seen[p] = struct{}{}
		b.WriteString(strconv.Itoa(p))
	}
	return b.String(), nil
}

func validateSingleExtensionSelection(selected []staging.StagedItem) error {
	extSet := make(map[string]struct{}, len(selected))
	for _, item := range selected {
		ext := strings.ToLower(filepath.Ext(item.File.Filename))
		if ext == "" {
			ext = strings.ToLower(filepath.Ext(item.File.Path))
		}
		if ext == "" {
			return fmt.Errorf("missing extension: %s", item.File.Filename)
		}
		extSet[ext] = struct{}{}
		if len(extSet) > 1 {
			return fmt.Errorf("multiple extensions selected")
		}
	}
	return nil
}

func hasMixedMKVAndMP4(items []staging.StagedItem) bool {
	hasMKV := false
	hasMP4 := false
	for _, item := range items {
		ext := strings.ToLower(filepath.Ext(item.File.Filename))
		if ext == "" {
			ext = strings.ToLower(filepath.Ext(item.File.Path))
		}
		switch ext {
		case ".mkv":
			hasMKV = true
		case ".mp4":
			hasMP4 = true
		}
		if hasMKV && hasMP4 {
			return true
		}
	}
	return false
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
