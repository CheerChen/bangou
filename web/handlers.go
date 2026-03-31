package web

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/CheerChen/bangou/committed"
	"github.com/CheerChen/bangou/executor"
	"github.com/CheerChen/bangou/provider"
	"github.com/CheerChen/bangou/staging"
)

type Handlers struct {
	staging     *staging.Manager
	store       committed.Store
	executor    *executor.Executor
	scanFn      func()
	rescrapeFn  func(string)
	libScrapeFn func(string) (*provider.MovieMetadata, map[string]string)
	aria2       Aria2Status

	libRescrape sync.Map // number -> *LibRescrapeResult

	linkAllMu sync.Mutex
	linkAll   *LinkAllStatus
}

type LinkAllStatus struct {
	Total   int      `json:"total"`
	Done    int      `json:"done"`
	Current string   `json:"current"`
	Errors  []string `json:"errors,omitempty"`
	Running bool     `json:"running"`
}

type LibRescrapeResult struct {
	Status string
	Old    *committed.Metadata
	New    *provider.MovieMetadata
	Errors map[string]string
}

// ── Pipelines ──

type PipelineResponse struct {
	ID               int64    `json:"id"`
	Name             string   `json:"name"`
	InputDir         string   `json:"inputDir"`
	OutputDir        string   `json:"outputDir"`
	PathPattern      string   `json:"pathPattern"`
	ArchiveDir       string   `json:"archiveDir"`
	EnableMerge      bool     `json:"enableMerge"`
	DownloadProvider string   `json:"downloadProvider"`
	ScrapeProviders  []string `json:"scrapeProviders"`
	PendingCount     int      `json:"pendingCount"`
	LibraryCount     int      `json:"libraryCount"`
	Status           string   `json:"status"` // "idle", "scanning"
}

func (h *Handlers) ListPipelines(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	pipes, err := h.store.ListPipelines(ctx)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	out := make([]PipelineResponse, 0, len(pipes))
	for _, p := range pipes {
		_, libCount, _ := h.store.ListOutputsByPipeline(ctx, p.ID, 0, 0)
		groups := h.staging.ListGroups()
		unknowns := h.staging.ListUnknowns()
		out = append(out, PipelineResponse{
			ID:               p.ID,
			Name:             p.Name,
			InputDir:         p.InputDir,
			OutputDir:        p.OutputDir,
			PathPattern:      p.PathPattern,
			ArchiveDir:       p.ArchiveDir,
			EnableMerge:      p.EnableMerge,
			DownloadProvider: p.DownloadProvider,
			ScrapeProviders:  splitProviders(p.ScrapeProviders),
			PendingCount:     len(groups) + len(unknowns),
			LibraryCount:     libCount,
			Status:           "idle",
		})
	}
	writeOK(w, out)
}

func (h *Handlers) CreatePipeline(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name             string   `json:"name"`
		InputDir         string   `json:"inputDir"`
		OutputDir        string   `json:"outputDir"`
		PathPattern      string   `json:"pathPattern"`
		ArchiveDir       string   `json:"archiveDir"`
		EnableMerge      bool     `json:"enableMerge"`
		DownloadProvider string   `json:"downloadProvider"`
		ScrapeProviders  []string `json:"scrapeProviders"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, 400, "invalid JSON")
		return
	}
	if req.Name == "" || req.InputDir == "" || req.OutputDir == "" {
		writeError(w, 400, "name, inputDir, outputDir are required")
		return
	}
	if req.PathPattern == "" {
		req.PathPattern = "{Number}"
	}
	if len(req.ScrapeProviders) == 0 {
		req.ScrapeProviders = []string{"avwiki", "dmm"}
	}

	p := &committed.Pipeline{
		Name:             req.Name,
		InputDir:         req.InputDir,
		OutputDir:        req.OutputDir,
		PathPattern:      req.PathPattern,
		ArchiveDir:       req.ArchiveDir,
		EnableMerge:      req.EnableMerge,
		DownloadProvider: req.DownloadProvider,
		ScrapeProviders:  strings.Join(req.ScrapeProviders, ","),
	}
	id, err := h.store.CreatePipeline(r.Context(), p)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			writeError(w, 409, "input directory already in use")
			return
		}
		writeError(w, 500, err.Error())
		return
	}
	writeJSON(w, 201, map[string]int64{"id": id})
}

func (h *Handlers) DeletePipeline(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, 400, "invalid id")
		return
	}
	if err := h.store.DeletePipeline(r.Context(), id); err != nil {
		writeError(w, 500, err.Error())
		return
	}
	writeOK(w, map[string]string{"status": "deleted"})
}

// ── Groups (Pending) ──

type GroupResponse struct {
	Number       string            `json:"number"`
	Items        []ItemResponse    `json:"items"`
	TotalSizeGB  float64           `json:"totalSizeGB"`
	Scrape       ScrapeResponse    `json:"scrape"`
	Task         string            `json:"task"`
	TaskErr      string            `json:"taskErr,omitempty"`
	TaskProgress int               `json:"taskProgress"`
	AllReady     bool              `json:"allReady"`
}

type ItemResponse struct {
	Path           string  `json:"path"`
	Filename       string  `json:"filename"`
	Part           int     `json:"part"`
	SizeGB         float64 `json:"sizeGB"`
	Ready          bool    `json:"ready"`
	Resolution     string  `json:"resolution,omitempty"`
	VideoCodec     string  `json:"videoCodec,omitempty"`
	AudioCodec     string  `json:"audioCodec,omitempty"`
	Bitrate        string  `json:"bitrate,omitempty"`
	Duration       string  `json:"duration,omitempty"`
	DownloadPct    int     `json:"downloadPct"`
	DownloadStatus string  `json:"downloadStatus,omitempty"`
}

type ScrapeResponse struct {
	Meta   *MetaResponse     `json:"meta"`
	Errors map[string]string `json:"errors,omitempty"`
	Status string            `json:"status"`
}

type MetaResponse struct {
	Number       string   `json:"number"`
	Title        string   `json:"title"`
	Maker        string   `json:"maker,omitempty"`
	Label        string   `json:"label,omitempty"`
	Series       string   `json:"series,omitempty"`
	Actors       []string `json:"actors,omitempty"`
	Genres       []string `json:"genres,omitempty"`
	CoverURL     string   `json:"coverURL,omitempty"`
	SampleImages []string `json:"sampleImages,omitempty"`
	Premiered    string   `json:"premiered,omitempty"`
	Year         string   `json:"year,omitempty"`
	Runtime      string   `json:"runtime,omitempty"`
	Rating       string   `json:"rating,omitempty"`
	ReviewCount  int      `json:"reviewCount"`
	PageURL      string   `json:"pageURL,omitempty"`
	Provider     string   `json:"provider,omitempty"`
}

type UnknownResponse struct {
	Path     string  `json:"path"`
	Filename string  `json:"filename"`
	SizeGB   float64 `json:"sizeGB"`
}

type GroupsPageResponse struct {
	Groups   []GroupResponse   `json:"groups"`
	Unknowns []UnknownResponse `json:"unknowns"`
	Linkable int               `json:"linkable"`
}

func (h *Handlers) ListGroups(w http.ResponseWriter, r *http.Request) {
	groups := h.staging.ListGroups()
	unknowns := h.staging.ListUnknowns()

	grs := make([]GroupResponse, 0, len(groups))
	linkable := 0
	for _, g := range groups {
		gr := buildGroupResponse(g)
		if gr.Scrape.Status == "success" && gr.AllReady && gr.Task == "" {
			linkable++
		}
		grs = append(grs, gr)
	}

	urs := make([]UnknownResponse, 0, len(unknowns))
	for _, u := range unknowns {
		urs = append(urs, UnknownResponse{
			Path:     u.Path,
			Filename: u.Filename,
			SizeGB:   float64(u.Size) / (1024 * 1024 * 1024),
		})
	}

	writeOK(w, GroupsPageResponse{Groups: grs, Unknowns: urs, Linkable: linkable})
}

func buildGroupResponse(g staging.StagingGroup) GroupResponse {
	gr := GroupResponse{
		Number:       g.Number,
		Task:         g.Task,
		TaskErr:      g.TaskErr,
		TaskProgress: g.TaskProgress,
		AllReady:     true,
	}

	var totalSize int64
	items := make([]ItemResponse, 0, len(g.Items))
	for _, item := range g.Items {
		totalSize += item.File.Size
		if !item.File.Ready {
			gr.AllReady = false
		}
		ir := ItemResponse{
			Path:           item.File.Path,
			Filename:       item.File.Filename,
			Part:           item.Parsed.Part,
			SizeGB:         float64(item.File.Size) / (1024 * 1024 * 1024),
			Ready:          item.File.Ready,
			DownloadPct:    item.File.DownloadPct,
			DownloadStatus: item.File.DownloadStatus,
		}
		if item.File.Media != nil {
			m := item.File.Media
			ir.Resolution = m.Resolution()
			ir.VideoCodec = m.VideoCodec
			ir.AudioCodec = m.AudioCodec
			ir.Bitrate = m.BitrateText()
			ir.Duration = m.DurationText()
		}
		items = append(items, ir)
	}
	gr.Items = items
	gr.TotalSizeGB = float64(totalSize) / (1024 * 1024 * 1024)

	gr.Scrape = ScrapeResponse{
		Errors: g.Scrape.Errors,
		Status: g.Scrape.Status,
	}
	if g.Scrape.Meta != nil {
		mm := g.Scrape.Meta
		gr.Scrape.Meta = &MetaResponse{
			Number:       mm.Number,
			Title:        mm.Title,
			Maker:        mm.Maker,
			Label:        mm.Label,
			Series:       mm.Series,
			Actors:       mm.Actors,
			Genres:       mm.Genres,
			CoverURL:     mm.CoverURL,
			SampleImages: mm.SampleImages,
			Premiered:    mm.Premiered,
			Year:         mm.Year,
			Runtime:      mm.Runtime,
			Rating:       mm.Rating,
			ReviewCount:  mm.ReviewCount,
			PageURL:      mm.PageURL,
			Provider:     mm.Provider,
		}
	}
	return gr
}

// ── Group Actions ──

func (h *Handlers) GroupLink(w http.ResponseWriter, r *http.Request) {
	number := strings.ToUpper(strings.TrimSpace(r.PathValue("number")))
	var req struct {
		Paths []string `json:"paths"`
	}
	if err := readJSON(r, &req); err != nil || len(req.Paths) == 0 {
		writeError(w, 400, "paths required")
		return
	}
	h.staging.SetTask(number, "linking", "")
	go func() {
		if err := h.executor.Link(context.Background(), number, req.Paths); err != nil {
			log.Printf("[link] %s: error: %v", number, err)
			h.staging.SetTask(number, "error", err.Error())
		}
	}()
	writeOK(w, map[string]string{"status": "linking"})
}

func (h *Handlers) GroupMerge(w http.ResponseWriter, r *http.Request) {
	number := strings.ToUpper(strings.TrimSpace(r.PathValue("number")))
	var req struct {
		Paths []string `json:"paths"`
	}
	if err := readJSON(r, &req); err != nil || len(req.Paths) < 2 {
		writeError(w, 400, "at least 2 paths required")
		return
	}
	h.staging.SetTask(number, "merging", "")
	go func() {
		if err := h.executor.Merge(context.Background(), number, req.Paths); err != nil {
			log.Printf("[merge] %s: error: %v", number, err)
			h.staging.SetTask(number, "error", err.Error())
			return
		}
		h.staging.SetTask(number, "", "")
		if h.scanFn != nil {
			h.scanFn()
		}
	}()
	writeOK(w, map[string]string{"status": "merging"})
}

func (h *Handlers) GroupIgnore(w http.ResponseWriter, r *http.Request) {
	number := strings.ToUpper(strings.TrimSpace(r.PathValue("number")))
	h.staging.SetIgnored(number, true)
	writeOK(w, map[string]string{"status": "ignored"})
}

func (h *Handlers) GroupRescrape(w http.ResponseWriter, r *http.Request) {
	number := strings.ToUpper(strings.TrimSpace(r.PathValue("number")))
	h.staging.SetScrapeStatus(number, "scraping", map[string]string{})
	if h.rescrapeFn != nil {
		h.rescrapeFn(number)
	}
	writeOK(w, map[string]string{"status": "scraping"})
}

func (h *Handlers) ManualTag(w http.ResponseWriter, r *http.Request) {
	number := strings.ToUpper(strings.TrimSpace(r.PathValue("number")))
	var req struct {
		Path string `json:"path"`
	}
	if err := readJSON(r, &req); err != nil || req.Path == "" {
		writeError(w, 400, "path required")
		return
	}
	h.staging.ManualTag(req.Path, number)
	writeOK(w, map[string]string{"status": "tagged"})
}

// ── Unknown File Actions ──

func (h *Handlers) UnknownTag(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path   string `json:"path"`
		Number string `json:"number"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, 400, "invalid JSON")
		return
	}
	req.Number = strings.ToUpper(strings.TrimSpace(req.Number))
	if req.Path == "" || req.Number == "" {
		writeError(w, 400, "path and number required")
		return
	}
	h.staging.ManualTag(req.Path, req.Number)
	writeOK(w, map[string]string{"status": "tagged"})
}

func (h *Handlers) UnknownIgnore(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path string `json:"path"`
	}
	if err := readJSON(r, &req); err != nil || req.Path == "" {
		writeError(w, 400, "path required")
		return
	}
	h.staging.SetUnknownIgnored(req.Path, true)
	writeOK(w, map[string]string{"status": "ignored"})
}

// ── Library ──

type LibraryItemResponse struct {
	ID           int64    `json:"id"`
	Number       string   `json:"number"`
	SrcPath      string   `json:"srcPath"`
	LinkPath     string   `json:"linkPath"`
	LinkType     string   `json:"linkType"`
	Alive        bool     `json:"alive"`
	Title        string   `json:"title,omitempty"`
	Actors       string   `json:"actors,omitempty"`
	Genres       []string `json:"genres,omitempty"`
	CoverURL     string   `json:"coverURL,omitempty"`
	SampleImages []string `json:"sampleImages,omitempty"`
	Rating       string   `json:"rating,omitempty"`
	ReviewCount  int      `json:"reviewCount"`
	PageURL      string   `json:"pageURL,omitempty"`
	Maker        string   `json:"maker,omitempty"`
	Year         string   `json:"year,omitempty"`
	Runtime      string   `json:"runtime,omitempty"`
	Provider     string   `json:"provider,omitempty"`
}

type LibraryPageResponse struct {
	Items []LibraryItemResponse `json:"items"`
	Total int                   `json:"total"`
	Page  int                   `json:"page"`
	Size  int                   `json:"size"`
}

func (h *Handlers) ListLibrary(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, 400, "invalid pipeline id")
		return
	}
	ctx := r.Context()
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	size, _ := strconv.Atoi(r.URL.Query().Get("size"))
	if size <= 0 || size > 100 {
		size = 12
	}
	if page < 0 {
		page = 0
	}

	outputs, total, err := h.store.ListOutputsByPipeline(ctx, id, size, page*size)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}

	metaCache := map[string]*committed.Metadata{}
	items := make([]LibraryItemResponse, 0, len(outputs))
	for _, o := range outputs {
		lv := LibraryItemResponse{
			ID: o.ID, Number: o.Number, SrcPath: o.SrcPath, LinkPath: o.LinkPath,
			LinkType: o.LinkType, Alive: o.Alive,
		}
		meta, ok := metaCache[o.Number]
		if !ok {
			meta, _ = h.store.GetMetadata(ctx, o.Number)
			metaCache[o.Number] = meta
		}
		if meta != nil {
			lv.Title = meta.Title
			lv.Actors = meta.Actors
			lv.CoverURL = meta.CoverURL
			lv.Provider = meta.Provider
			lv.Rating = meta.Rating
			lv.ReviewCount = meta.ReviewCount
			lv.PageURL = meta.PageURL
			lv.Maker = meta.Maker
			lv.Year = meta.Year
			lv.Runtime = meta.Runtime
			if meta.Genres != "" {
				lv.Genres = strings.Split(meta.Genres, ",")
			}
			if meta.SampleImages != "" {
				lv.SampleImages = strings.Split(meta.SampleImages, ",")
			}
		}
		items = append(items, lv)
	}
	writeOK(w, LibraryPageResponse{Items: items, Total: total, Page: page, Size: size})
}

// ── Scan / Link All ──

func (h *Handlers) TriggerScan(w http.ResponseWriter, r *http.Request) {
	if h.scanFn != nil {
		go h.scanFn()
	}
	writeOK(w, map[string]string{"status": "scanning"})
}

func (h *Handlers) LinkAll(w http.ResponseWriter, r *http.Request) {
	h.linkAllMu.Lock()
	if h.linkAll != nil && h.linkAll.Running {
		h.linkAllMu.Unlock()
		writeOK(w, h.linkAll)
		return
	}

	groups := h.staging.ListGroups()
	var eligible []string
	for _, g := range groups {
		if g.Scrape.Status != "success" || g.Task != "" {
			continue
		}
		allReady := true
		for _, item := range g.Items {
			if !item.File.Ready {
				allReady = false
				break
			}
		}
		if allReady {
			eligible = append(eligible, g.Number)
		}
	}

	if len(eligible) == 0 {
		h.linkAllMu.Unlock()
		writeError(w, 400, "no eligible groups")
		return
	}

	h.linkAll = &LinkAllStatus{Total: len(eligible), Running: true}
	h.linkAllMu.Unlock()

	go func() {
		for i, number := range eligible {
			h.linkAllMu.Lock()
			h.linkAll.Done = i
			h.linkAll.Current = number
			h.linkAllMu.Unlock()

			group := h.staging.GetGroup(number)
			if group == nil {
				continue
			}
			var paths []string
			for _, item := range group.Items {
				if item.File.Ready {
					paths = append(paths, item.File.Path)
				}
			}
			h.staging.SetTask(number, "linking", "")
			if err := h.executor.Link(context.Background(), number, paths); err != nil {
				log.Printf("[link-all] %s: error: %v", number, err)
				h.staging.SetTask(number, "error", err.Error())
				h.linkAllMu.Lock()
				h.linkAll.Errors = append(h.linkAll.Errors, fmt.Sprintf("%s: %s", number, err.Error()))
				h.linkAllMu.Unlock()
			}
		}
		h.linkAllMu.Lock()
		h.linkAll.Done = len(eligible)
		h.linkAll.Current = ""
		h.linkAll.Running = false
		h.linkAllMu.Unlock()
	}()

	writeOK(w, h.linkAll)
}

func (h *Handlers) LinkAllProgress(w http.ResponseWriter, r *http.Request) {
	h.linkAllMu.Lock()
	s := h.linkAll
	h.linkAllMu.Unlock()
	if s == nil {
		writeOK(w, map[string]any{"running": false})
		return
	}
	writeOK(w, s)
}

// ── Library Rescrape ──

func (h *Handlers) LibraryRescrape(w http.ResponseWriter, r *http.Request) {
	number := strings.ToUpper(strings.TrimSpace(r.PathValue("number")))
	if _, loaded := h.libRescrape.LoadOrStore(number, &LibRescrapeResult{Status: "scraping"}); loaded {
		writeError(w, 409, "rescrape already in progress")
		return
	}
	go func() {
		meta, errs := h.libScrapeFn(number)
		if meta != nil {
			old, _ := h.store.GetMetadata(context.Background(), number)
			h.libRescrape.Store(number, &LibRescrapeResult{Status: "done", Old: old, New: meta, Errors: errs})
		} else {
			h.libRescrape.Store(number, &LibRescrapeResult{Status: "failed", Errors: errs})
		}
	}()
	writeOK(w, map[string]string{"status": "scraping"})
}

func (h *Handlers) LibraryRescrapeApply(w http.ResponseWriter, r *http.Request) {
	number := strings.ToUpper(strings.TrimSpace(r.PathValue("number")))
	v, ok := h.libRescrape.Load(number)
	if !ok {
		writeError(w, 404, "no pending rescrape")
		return
	}
	res := v.(*LibRescrapeResult)
	if res.Status != "done" || res.New == nil {
		writeError(w, 400, "rescrape not ready")
		return
	}
	ctx := r.Context()
	_ = h.store.UpsertMetadata(ctx, &committed.Metadata{
		Number: number, Title: res.New.Title, Plot: res.New.Plot,
		Director: res.New.Director, Maker: res.New.Maker, Label: res.New.Label,
		Series: res.New.Series, Actors: strings.Join(res.New.Actors, ","),
		Genres: strings.Join(res.New.Genres, ","), CoverURL: res.New.CoverURL,
		SampleImages: strings.Join(res.New.SampleImages, ","),
		Premiered: res.New.Premiered, Year: res.New.Year, Runtime: res.New.Runtime,
		Rating: res.New.Rating, ReviewCount: res.New.ReviewCount,
		PageURL: res.New.PageURL, ContentID: res.New.ContentID, Provider: res.New.Provider,
	})
	h.libRescrape.Delete(number)
	writeOK(w, map[string]string{"status": "applied"})
}

func (h *Handlers) LibraryRescrapeDismiss(w http.ResponseWriter, r *http.Request) {
	number := strings.ToUpper(strings.TrimSpace(r.PathValue("number")))
	h.libRescrape.Delete(number)
	writeOK(w, map[string]string{"status": "dismissed"})
}

// ── Output Actions ──

func (h *Handlers) UnlinkOutput(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, 400, "invalid id")
		return
	}
	var req struct {
		Number string `json:"number"`
	}
	_ = readJSON(r, &req)
	number := strings.ToUpper(strings.TrimSpace(req.Number))
	if err := h.executor.Unlink(r.Context(), number, id); err != nil {
		writeError(w, 500, err.Error())
		return
	}
	if h.scanFn != nil {
		go h.scanFn()
	}
	writeOK(w, map[string]string{"status": "unlinked"})
}

func (h *Handlers) DeleteOutput(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, 400, "invalid id")
		return
	}
	if err := h.store.DeleteOutput(r.Context(), id); err != nil {
		writeError(w, 500, err.Error())
		return
	}
	writeOK(w, map[string]string{"status": "deleted"})
}

// ── Provider Configs ──

func (h *Handlers) ListProviderConfigs(w http.ResponseWriter, r *http.Request) {
	configs, err := h.store.ListProviderConfigs(r.Context())
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	writeOK(w, configs)
}

func (h *Handlers) SetProviderConfig(w http.ResponseWriter, r *http.Request) {
	prov := r.PathValue("provider")
	var req map[string]any
	if err := readJSON(r, &req); err != nil {
		writeError(w, 400, "invalid JSON")
		return
	}
	raw, _ := encodeJSON(req)
	if err := h.store.SetProviderConfig(r.Context(), prov, raw); err != nil {
		writeError(w, 500, err.Error())
		return
	}
	writeOK(w, map[string]string{"status": "saved"})
}

func (h *Handlers) TestProviderConfig(w http.ResponseWriter, r *http.Request) {
	prov := r.PathValue("provider")
	ctx := r.Context()

	switch prov {
	case "dmm":
		cfgStr, _ := h.store.GetProviderConfig(ctx, "dmm")
		var cfg struct {
			APIID       string `json:"api_id"`
			AffiliateID string `json:"affiliate_id"`
		}
		_ = decodeJSON(cfgStr, &cfg)
		if cfg.APIID == "" || cfg.AffiliateID == "" {
			writeError(w, 400, "DMM API ID and Affiliate ID required")
			return
		}
		p := provider.NewDMM(cfg.APIID, cfg.AffiliateID)
		_, err := p.Scrape(ctx, provider.Predict{Number: "SIVR-476"})
		if err != nil {
			writeError(w, 400, err.Error())
			return
		}
		writeOK(w, map[string]string{"status": "ok"})

	case "aria2":
		// TODO: test aria2 connection
		writeOK(w, map[string]string{"status": "ok"})

	default:
		writeError(w, 400, "unknown provider")
	}
}

// ── Helpers ──

func splitProviders(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

