package web

import (
	"context"
	"encoding/json"
	"log"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/CheerChen/bangou/committed"
	"github.com/CheerChen/bangou/executor"
	"github.com/CheerChen/bangou/parser"
	"github.com/CheerChen/bangou/provider"
	"github.com/CheerChen/bangou/scanner"
	"github.com/CheerChen/bangou/staging"
)

// PipelineRuntime holds per-pipeline state.
type PipelineRuntime struct {
	Pipeline committed.Pipeline
	Manager  *staging.Manager
	Executor *executor.Executor

	scrapeQueue chan scrapeJob
	mu          sync.Mutex
	inFlight    map[string]bool
	cancel      context.CancelFunc
}

type scrapeJob struct {
	number string
	force  bool
}

// Registry manages all pipeline runtimes.
type Registry struct {
	mu       sync.RWMutex
	runtimes map[int64]*PipelineRuntime
	store    committed.Store
	rootCtx  context.Context
}

func NewRegistry(ctx context.Context, store committed.Store) *Registry {
	return &Registry{
		runtimes: make(map[int64]*PipelineRuntime),
		store:    store,
		rootCtx:  ctx,
	}
}

func (reg *Registry) Get(pipelineID int64) *PipelineRuntime {
	reg.mu.RLock()
	defer reg.mu.RUnlock()
	return reg.runtimes[pipelineID]
}

func (reg *Registry) All() []*PipelineRuntime {
	reg.mu.RLock()
	defer reg.mu.RUnlock()
	out := make([]*PipelineRuntime, 0, len(reg.runtimes))
	for _, rt := range reg.runtimes {
		out = append(out, rt)
	}
	return out
}

// StartPipeline creates and starts a runtime for the given pipeline.
func (reg *Registry) StartPipeline(p committed.Pipeline) error {
	reg.mu.Lock()
	defer reg.mu.Unlock()

	if _, exists := reg.runtimes[p.ID]; exists {
		return nil // already running
	}

	ctx, cancel := context.WithCancel(reg.rootCtx)
	mgr := staging.New()
	exec := executor.New(reg.store, mgr, p.OutputDir)

	rt := &PipelineRuntime{
		Pipeline:    p,
		Manager:     mgr,
		Executor:    exec,
		scrapeQueue: make(chan scrapeJob, 256),
		inFlight:    make(map[string]bool),
		cancel:      cancel,
	}

	mgr.OnNewNumber = func(number string) {
		rt.enqueue(number, false)
	}

	// Scrape worker
	go rt.scrapeWorker(ctx, reg.store)

	// Initial scan
	go rt.scan(ctx, reg.store)

	reg.runtimes[p.ID] = rt
	log.Printf("[pipeline] started: %s (id=%d, input=%s)", p.Name, p.ID, p.InputDir)
	return nil
}

// StopPipeline stops and removes a pipeline runtime.
func (reg *Registry) StopPipeline(pipelineID int64) {
	reg.mu.Lock()
	rt, ok := reg.runtimes[pipelineID]
	if ok {
		delete(reg.runtimes, pipelineID)
	}
	reg.mu.Unlock()

	if ok && rt.cancel != nil {
		rt.cancel()
		log.Printf("[pipeline] stopped: %s (id=%d)", rt.Pipeline.Name, pipelineID)
	}
}

// FindByInputDir returns the runtime whose inputDir matches the given path.
func (reg *Registry) FindByInputDir(dir string) *PipelineRuntime {
	reg.mu.RLock()
	defer reg.mu.RUnlock()
	for _, rt := range reg.runtimes {
		if rt.Pipeline.InputDir == dir {
			return rt
		}
	}
	return nil
}

// ── PipelineRuntime methods ──

func (rt *PipelineRuntime) enqueue(number string, force bool) {
	number = strings.ToUpper(strings.TrimSpace(number))
	if number == "" {
		return
	}
	rt.mu.Lock()
	if rt.inFlight[number] {
		rt.mu.Unlock()
		return
	}
	rt.inFlight[number] = true
	rt.mu.Unlock()

	select {
	case rt.scrapeQueue <- scrapeJob{number: number, force: force}:
	default:
		rt.mu.Lock()
		delete(rt.inFlight, number)
		rt.mu.Unlock()
		log.Printf("[pipeline:%s] scrape queue full, drop %s", rt.Pipeline.Name, number)
	}
}

func (rt *PipelineRuntime) Rescrape(number string) {
	rt.Manager.SetScrapeStatus(number, "scraping", map[string]string{})
	rt.enqueue(number, true)
}

func (rt *PipelineRuntime) Scan(ctx context.Context, store committed.Store) {
	rt.scan(ctx, store)
}

func (rt *PipelineRuntime) scan(ctx context.Context, store committed.Store) {
	inputDir := rt.Pipeline.InputDir
	if inputDir == "" {
		return
	}
	files, err := scanner.ScanDir(inputDir)
	if err != nil {
		log.Printf("[pipeline:%s] scan failed: %v", rt.Pipeline.Name, err)
		return
	}
	paths := make(map[string]bool, len(files))
	for _, f := range files {
		paths[f.Path] = true

		parsed := parser.Parse(f.Filename)
		if parsed.Number != "" {
			if ok, _ := store.IsCommitted(ctx, parsed.Number); ok {
				continue
			}
		}

		if !f.Ready {
			rt.Manager.Ingest(staging.StagingFile{Path: f.Path, Filename: f.Filename, Size: f.Size, Ready: f.Ready})
			continue
		}
		var media *scanner.MediaInfo
		if rt.Manager.HasMedia(f.Path, f.Size) {
			// cached
		} else {
			media, err = scanner.Probe(f.Path)
			if err != nil {
				log.Printf("[pipeline:%s] probe %s: %v", rt.Pipeline.Name, f.Filename, err)
			} else if media != nil && f.Size > 0 && media.Duration > 0 {
				media.BitrateBps = int64(float64(f.Size*8) / media.Duration)
			}
		}
		rt.Manager.Ingest(staging.StagingFile{Path: f.Path, Filename: f.Filename, Size: f.Size, Ready: f.Ready, Media: media})
	}
	rt.Manager.Reconcile(paths)
	log.Printf("[pipeline:%s] scan complete: %d files", rt.Pipeline.Name, len(files))
}

func (rt *PipelineRuntime) scrapeWorker(ctx context.Context, store committed.Store) {
	for {
		select {
		case <-ctx.Done():
			return
		case job := <-rt.scrapeQueue:
			rt.processScrapeJob(ctx, store, job)
			rt.mu.Lock()
			delete(rt.inFlight, job.number)
			rt.mu.Unlock()
		}
	}
}

func (rt *PipelineRuntime) processScrapeJob(ctx context.Context, store committed.Store, job scrapeJob) {
	number := job.number
	if !job.force {
		if ok, _ := store.IsCommitted(ctx, number); ok {
			if meta, _ := store.GetMetadata(ctx, number); meta != nil {
				rt.Manager.SetScrapeResult(number, staging.ScrapeResult{
					Meta:   metadataToMovie(meta),
					Errors: map[string]string{},
					Status: "success",
				})
			}
			return
		}
	}

	rt.Manager.SetScrapeStatus(number, "scraping", map[string]string{})

	scrapeCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()

	providers := rt.buildProviders(scrapeCtx, store)
	if len(providers) == 0 {
		rt.Manager.SetScrapeResult(number, staging.ScrapeResult{Status: "failed", Errors: map[string]string{"system": "no provider configured"}})
		return
	}
	predict := provider.Predict{Number: number, RawNumber: rt.Manager.GetRawNumber(number)}
	result := provider.Chain(scrapeCtx, providers, predict)
	if result.Meta != nil {
		rt.Manager.SetScrapeResult(number, staging.ScrapeResult{Meta: result.Meta, Errors: result.Errors, Status: "success"})
		log.Printf("[pipeline:%s] scrape success: %s -> %s (%s)", rt.Pipeline.Name, number, result.Meta.Title, result.Meta.Provider)
		return
	}
	rt.Manager.SetScrapeResult(number, staging.ScrapeResult{Meta: nil, Errors: result.Errors, Status: "failed"})
	log.Printf("[pipeline:%s] scrape failed: %s", rt.Pipeline.Name, number)
}

func (rt *PipelineRuntime) buildProviders(ctx context.Context, store committed.Store) []provider.Provider {
	order := rt.Pipeline.ScrapeProviders
	if order == "" {
		order = "avwiki,dmm"
	}
	var providers []provider.Provider
	for _, name := range strings.Split(order, ",") {
		name = strings.TrimSpace(strings.ToLower(name))
		switch name {
		case "avwiki":
			providers = append(providers, provider.NewAVWiki())
		case "dmm":
			cfgStr, _ := store.GetProviderConfig(ctx, "dmm")
			var cfg struct {
				APIID       string `json:"api_id"`
				AffiliateID string `json:"affiliate_id"`
			}
			_ = json.Unmarshal([]byte(cfgStr), &cfg)
			if cfg.APIID != "" && cfg.AffiliateID != "" {
				providers = append(providers, provider.NewDMM(cfg.APIID, cfg.AffiliateID))
			}
		}
	}
	return providers
}

// LibScrapeFn returns a standalone scrape function for library rescrape (uses first pipeline's providers as fallback).
func (rt *PipelineRuntime) LibScrapeFn(ctx context.Context, store committed.Store, number string) (*provider.MovieMetadata, map[string]string) {
	scrapeCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	providers := rt.buildProviders(scrapeCtx, store)
	if len(providers) == 0 {
		return nil, map[string]string{"system": "no provider configured"}
	}
	result := provider.Chain(scrapeCtx, providers, provider.Predict{Number: number})
	return result.Meta, result.Errors
}

// ── Aria2 integration ──

// SetupAria2 creates and runs an aria2 client that distributes download progress to the matching pipeline.
func (reg *Registry) SetupAria2(ctx context.Context, store committed.Store) {
	cfgStr, _ := store.GetProviderConfig(ctx, "aria2")
	var cfg struct {
		RPCURL string `json:"rpc_url"`
		Token  string `json:"token"`
	}
	_ = json.Unmarshal([]byte(cfgStr), &cfg)
	if cfg.RPCURL == "" {
		log.Printf("aria2: not configured, skipping")
		return
	}

	scanAll := func() {
		for _, rt := range reg.All() {
			go rt.scan(ctx, store)
		}
	}

	aria2 := scanner.NewAria2Client(cfg.RPCURL, cfg.Token, scanAll, func(downloads []scanner.DownloadProgress) {
		for _, rt := range reg.All() {
			activeFilenames := make(map[string]bool)
			for _, d := range downloads {
				// Match download to pipeline by checking if path is under its inputDir
				if strings.HasPrefix(d.Path, rt.Pipeline.InputDir) {
					fn := filepath.Base(d.Path)
					activeFilenames[fn] = true
					rt.Manager.SetDownloadProgress(fn, d.Pct, d.Completed, d.Status)
				}
			}
			rt.Manager.ClearDownloadProgress(activeFilenames)
		}
	})
	go aria2.Run(ctx)
}

// ── Helpers ──

func metadataToMovie(m *committed.Metadata) *provider.MovieMetadata {
	return &provider.MovieMetadata{
		Number:       m.Number,
		Title:        m.Title,
		Plot:         m.Plot,
		Director:     m.Director,
		Maker:        m.Maker,
		Label:        m.Label,
		Series:       m.Series,
		Actors:       splitCSV(m.Actors),
		Genres:       splitCSV(m.Genres),
		CoverURL:     m.CoverURL,
		SampleImages: splitCSV(m.SampleImages),
		Premiered:    m.Premiered,
		Year:         m.Year,
		Runtime:      m.Runtime,
		Rating:       m.Rating,
		ReviewCount:  m.ReviewCount,
		PageURL:      m.PageURL,
		ContentID:    m.ContentID,
		Provider:     m.Provider,
	}
}

func splitCSV(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	raw := strings.Split(s, ",")
	out := make([]string, 0, len(raw))
	for _, p := range raw {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
