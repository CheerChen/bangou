package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/CheerChen/bangou/checker"
	"github.com/CheerChen/bangou/committed"
	"github.com/CheerChen/bangou/config"
	"github.com/CheerChen/bangou/executor"
	"github.com/CheerChen/bangou/parser"
	"github.com/CheerChen/bangou/provider"
	"github.com/CheerChen/bangou/scanner"
	"github.com/CheerChen/bangou/staging"
	"github.com/CheerChen/bangou/web"
)

type scrapeJob struct {
	number string
	force  bool
}

func main() {
	cfg := config.Parse()

	db, err := committed.NewSQLite(cfg.DBPath)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	settings, err := db.GetAllSettings(ctx)
	if err != nil {
		log.Fatalf("load settings: %v", err)
	}
	inputDir := config.ResolveSetting(settings["input_dir"], config.EnvInputDir, "/input")
	outputDir := config.ResolveSetting(settings["output_dir"], config.EnvOutputDir, "/output")

	mgr := staging.New()
	exec := executor.New(db, mgr, outputDir)
	scrapeQueue := make(chan scrapeJob, 256)

	var mu sync.Mutex
	inFlight := map[string]bool{}
	enqueue := func(number string, force bool) {
		number = strings.ToUpper(strings.TrimSpace(number))
		if number == "" {
			return
		}
		mu.Lock()
		if inFlight[number] {
			mu.Unlock()
			return
		}
		inFlight[number] = true
		mu.Unlock()

		select {
		case scrapeQueue <- scrapeJob{number: number, force: force}:
		default:
			mu.Lock()
			delete(inFlight, number)
			mu.Unlock()
			log.Printf("scrape queue full, drop %s", number)
		}
	}

	mgr.OnNewNumber = func(number string) {
		enqueue(number, false)
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case job := <-scrapeQueue:
				processScrapeJob(ctx, db, mgr, job)
				mu.Lock()
				delete(inFlight, job.number)
				mu.Unlock()
			}
		}
	}()

	scanNow := func() {
		if inputDir == "" {
			log.Printf("input_dir not configured yet. Use /settings")
			return
		}
		files, err := scanner.ScanDir(inputDir)
		if err != nil {
			log.Printf("scan failed: %v", err)
			return
		}
		paths := make(map[string]bool, len(files))
		for _, f := range files {
			paths[f.Path] = true

			// Skip files whose number is already committed
			parsed := parser.Parse(f.Filename)
			if parsed.Number != "" {
				if ok, _ := db.IsCommitted(ctx, parsed.Number); ok {
					continue
				}
			}

			if !f.Ready {
				mgr.Ingest(staging.StagingFile{Path: f.Path, Filename: f.Filename, Size: f.Size, Ready: f.Ready})
				continue
			}
			var media *scanner.MediaInfo
			if mgr.HasMedia(f.Path, f.Size) {
				// Already probed and size unchanged, skip
			} else {
				media, err = scanner.Probe(f.Path)
				if err != nil {
					log.Printf("probe %s: %v", f.Filename, err)
				} else if media != nil && f.Size > 0 && media.Duration > 0 {
					media.BitrateBps = int64(float64(f.Size*8) / media.Duration)
				}
			}
			mgr.Ingest(staging.StagingFile{Path: f.Path, Filename: f.Filename, Size: f.Size, Ready: f.Ready, Media: media})
		}
		mgr.Reconcile(paths)
		log.Printf("scan complete: %d files found", len(files))
	}
	rescrapeNow := func(number string) {
		mgr.SetScrapeStatus(number, "scraping", map[string]string{})
		enqueue(number, true)
	}

	// Initial scan on startup
	scanNow()

	// aria2 RPC: connect and watch for completed downloads
	aria2URL := config.ResolveSetting(settings["aria2_rpc_url"], config.EnvAria2RPCURL, "")
	aria2Token := config.ResolveSetting(settings["aria2_token"], config.EnvAria2Token, "")
	var aria2 *scanner.Aria2Client
	if aria2URL != "" {
		aria2 = scanner.NewAria2Client(aria2URL, aria2Token, scanNow, func(downloads []scanner.DownloadProgress) {
			activeFilenames := make(map[string]bool, len(downloads))
			for _, d := range downloads {
				fn := filepath.Base(d.Path)
				activeFilenames[fn] = true
				mgr.SetDownloadProgress(fn, d.Pct, d.Completed, d.Status)
			}
			mgr.ClearDownloadProgress(activeFilenames)
		})
		go aria2.Run(ctx)
	} else {
		log.Printf("aria2: not configured, skipping")
	}

	go checker.Run(ctx, db, time.Hour)

	libScrapeFn := func(number string) (*provider.MovieMetadata, map[string]string) {
		scrapeCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
		defer cancel()
		providers := buildProviders(scrapeCtx, db)
		if len(providers) == 0 {
			return nil, map[string]string{"system": "no provider configured"}
		}
		result := provider.Chain(scrapeCtx, providers, number)
		return result.Meta, result.Errors
	}

	var aria2Status web.Aria2Status
	if aria2 != nil {
		aria2Status = aria2
	}
	srv := web.NewServer(mgr, db, exec, scanNow, rescrapeNow, libScrapeFn, aria2Status)
	go func() {
		log.Printf("web ui: http://%s", cfg.ListenAddr)
		if err := http.ListenAndServe(cfg.ListenAddr, srv); err != nil {
			log.Fatalf("listen: %v", err)
		}
	}()

	<-ctx.Done()
}

func processScrapeJob(ctx context.Context, db committed.Store, mgr *staging.Manager, job scrapeJob) {
	number := job.number
	if !job.force {
		if ok, _ := db.IsCommitted(ctx, number); ok {
			if meta, _ := db.GetMetadata(ctx, number); meta != nil {
				mgr.SetScrapeResult(number, staging.ScrapeResult{
					Meta: &provider.MovieMetadata{
						Number:       meta.Number,
						Title:        meta.Title,
						Plot:         meta.Plot,
						Director:     meta.Director,
						Maker:        meta.Maker,
						Label:        meta.Label,
						Series:       meta.Series,
						Actors:       splitCSV(meta.Actors),
						Genres:       splitCSV(meta.Genres),
						CoverURL:     meta.CoverURL,
						SampleImages: splitCSV(meta.SampleImages),
						Premiered:    meta.Premiered,
						Year:         meta.Year,
						Runtime:      meta.Runtime,
						Rating:       meta.Rating,
						ReviewCount:  meta.ReviewCount,
						PageURL:      meta.PageURL,
						ContentID:    meta.ContentID,
						Provider:     meta.Provider,
					},
					Errors: map[string]string{},
					Status: "success",
				})
			}
			return
		}
	}

	mgr.SetScrapeStatus(number, "scraping", map[string]string{})

	scrapeCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	providers := buildProviders(scrapeCtx, db)
	if len(providers) == 0 {
		mgr.SetScrapeResult(number, staging.ScrapeResult{Status: "failed", Errors: map[string]string{"system": "no provider configured"}})
		return
	}
	result := provider.Chain(scrapeCtx, providers, number)
	if result.Meta != nil {
		mgr.SetScrapeResult(number, staging.ScrapeResult{Meta: result.Meta, Errors: result.Errors, Status: "success"})
		log.Printf("scrape success: %s -> %s (%s)", number, result.Meta.Title, result.Meta.Provider)
		return
	}
	mgr.SetScrapeResult(number, staging.ScrapeResult{Meta: nil, Errors: result.Errors, Status: "failed"})
	log.Printf("scrape failed: %s", number)
}

func buildProviders(ctx context.Context, db committed.Store) []provider.Provider {
	settings, err := db.GetAllSettings(ctx)
	if err != nil {
		return []provider.Provider{provider.NewAVWiki()}
	}
	order := strings.TrimSpace(settings["provider_order"])
	if order == "" {
		order = "avwiki,dmm"
	}
	var providers []provider.Provider
	for _, name := range strings.Split(order, ",") {
		name = strings.TrimSpace(strings.ToLower(name))
		if name == "" {
			continue
		}
		if settings["provider_enabled_"+name] == "false" {
			continue
		}
		switch name {
		case "avwiki":
			providers = append(providers, provider.NewAVWiki())
		case "dmm":
			apiID := strings.TrimSpace(settings["dmm_api_id"])
			affID := strings.TrimSpace(settings["dmm_affiliate_id"])
			if apiID != "" && affID != "" {
				providers = append(providers, provider.NewDMM(apiID, affID))
			}
		}
	}
	return providers
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
