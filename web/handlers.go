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
	"github.com/CheerChen/bangou/provider"
	"github.com/CheerChen/bangou/staging"
)

type Handlers struct {
	registry *Registry
	store    committed.Store

	libRescrape sync.Map // number -> *LibRescrapeResult

	// per-pipeline link-all state
	linkAllMu sync.Mutex
	linkAll   map[int64]*LinkAllStatus
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

// helper to resolve pipeline runtime from URL path
func (h *Handlers) getRuntime(r *http.Request) (*PipelineRuntime, int64, error) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		return nil, 0, fmt.Errorf("invalid pipeline id")
	}
	rt := h.registry.Get(id)
	if rt == nil {
		return nil, id, fmt.Errorf("pipeline %d not found", id)
	}
	return rt, id, nil
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
	Status           string   `json:"status"`
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
		_, libCount, _ := h.store.ListOutputsByPipeline(ctx, p.ID, 0, 0, "", "")
		pending := 0
		if rt := h.registry.Get(p.ID); rt != nil {
			pending = len(rt.Manager.ListGroups()) + len(rt.Manager.ListUnknowns())
		}
		out = append(out, PipelineResponse{
			ID: p.ID, Name: p.Name, InputDir: p.InputDir, OutputDir: p.OutputDir,
			PathPattern: p.PathPattern, ArchiveDir: p.ArchiveDir, EnableMerge: p.EnableMerge,
			DownloadProvider: p.DownloadProvider, ScrapeProviders: splitProviders(p.ScrapeProviders),
			PendingCount: pending, LibraryCount: libCount, Status: "idle",
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
		writeError(w, 400, "name, inputDir, outputDir required")
		return
	}
	if req.PathPattern == "" {
		req.PathPattern = "{Number}"
	}
	if len(req.ScrapeProviders) == 0 {
		req.ScrapeProviders = []string{"avwiki", "dmm"}
	}

	p := &committed.Pipeline{
		Name: req.Name, InputDir: req.InputDir, OutputDir: req.OutputDir,
		PathPattern: req.PathPattern, ArchiveDir: req.ArchiveDir, EnableMerge: req.EnableMerge,
		DownloadProvider: req.DownloadProvider, ScrapeProviders: strings.Join(req.ScrapeProviders, ","),
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
	p.ID = id
	_ = h.registry.StartPipeline(*p)
	writeJSON(w, 201, map[string]int64{"id": id})
}

func (h *Handlers) DeletePipeline(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, 400, "invalid id")
		return
	}
	h.registry.StopPipeline(id)
	if err := h.store.DeletePipeline(r.Context(), id); err != nil {
		writeError(w, 500, err.Error())
		return
	}
	writeOK(w, map[string]string{"status": "deleted"})
}

// ── Groups ──

type GroupResponse struct {
	Number       string         `json:"number"`
	Items        []ItemResponse `json:"items"`
	TotalSizeGB  float64        `json:"totalSizeGB"`
	Scrape       ScrapeResponse `json:"scrape"`
	Task         string         `json:"task"`
	TaskErr      string         `json:"taskErr,omitempty"`
	TaskProgress int            `json:"taskProgress"`
	AllReady     bool           `json:"allReady"`
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
	rt, _, err := h.getRuntime(r)
	if err != nil {
		writeError(w, 404, err.Error())
		return
	}
	groups := rt.Manager.ListGroups()
	unknowns := rt.Manager.ListUnknowns()

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
		urs = append(urs, UnknownResponse{Path: u.Path, Filename: u.Filename, SizeGB: float64(u.Size) / (1024 * 1024 * 1024)})
	}
	writeOK(w, GroupsPageResponse{Groups: grs, Unknowns: urs, Linkable: linkable})
}

func buildGroupResponse(g staging.StagingGroup) GroupResponse {
	gr := GroupResponse{Number: g.Number, Task: g.Task, TaskErr: g.TaskErr, TaskProgress: g.TaskProgress, AllReady: true}
	var totalSize int64
	items := make([]ItemResponse, 0, len(g.Items))
	for _, item := range g.Items {
		totalSize += item.File.Size
		if !item.File.Ready {
			gr.AllReady = false
		}
		ir := ItemResponse{
			Path: item.File.Path, Filename: item.File.Filename, Part: item.Parsed.Part,
			SizeGB: float64(item.File.Size) / (1024 * 1024 * 1024), Ready: item.File.Ready,
			DownloadPct: item.File.DownloadPct, DownloadStatus: item.File.DownloadStatus,
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
	gr.Scrape = ScrapeResponse{Errors: g.Scrape.Errors, Status: g.Scrape.Status}
	if g.Scrape.Meta != nil {
		mm := g.Scrape.Meta
		gr.Scrape.Meta = &MetaResponse{
			Number: mm.Number, Title: mm.Title, Maker: mm.Maker, Label: mm.Label,
			Series: mm.Series, Actors: mm.Actors, Genres: mm.Genres, CoverURL: mm.CoverURL,
			SampleImages: mm.SampleImages, Premiered: mm.Premiered, Year: mm.Year,
			Runtime: mm.Runtime, Rating: mm.Rating, ReviewCount: mm.ReviewCount,
			PageURL: mm.PageURL, Provider: mm.Provider,
		}
	}
	return gr
}

// ── Group Actions ──

func (h *Handlers) GroupLink(w http.ResponseWriter, r *http.Request) {
	rt, _, err := h.getRuntime(r)
	if err != nil {
		writeError(w, 404, err.Error())
		return
	}
	number := strings.ToUpper(strings.TrimSpace(r.PathValue("number")))
	var req struct {
		Paths []string `json:"paths"`
	}
	if err := readJSON(r, &req); err != nil || len(req.Paths) == 0 {
		writeError(w, 400, "paths required")
		return
	}
	rt.Manager.SetTask(number, "linking", "")
	go func() {
		if err := rt.Executor.Link(context.Background(), number, req.Paths, rt.LinkOpts()); err != nil {
			log.Printf("[link] %s: error: %v", number, err)
			rt.Manager.SetTask(number, "error", err.Error())
		}
	}()
	writeOK(w, map[string]string{"status": "linking"})
}

func (h *Handlers) GroupMerge(w http.ResponseWriter, r *http.Request) {
	rt, _, err := h.getRuntime(r)
	if err != nil {
		writeError(w, 404, err.Error())
		return
	}
	number := strings.ToUpper(strings.TrimSpace(r.PathValue("number")))
	var req struct {
		Paths []string `json:"paths"`
	}
	if err := readJSON(r, &req); err != nil || len(req.Paths) < 2 {
		writeError(w, 400, "at least 2 paths required")
		return
	}
	rt.Manager.SetTask(number, "merging", "")
	go func() {
		if err := rt.Executor.Merge(context.Background(), number, req.Paths); err != nil {
			log.Printf("[merge] %s: error: %v", number, err)
			rt.Manager.SetTask(number, "error", err.Error())
			return
		}
		rt.Manager.SetTask(number, "", "")
		go rt.Scan(context.Background(), h.store)
	}()
	writeOK(w, map[string]string{"status": "merging"})
}

func (h *Handlers) GroupIgnore(w http.ResponseWriter, r *http.Request) {
	rt, _, err := h.getRuntime(r)
	if err != nil {
		writeError(w, 404, err.Error())
		return
	}
	number := strings.ToUpper(strings.TrimSpace(r.PathValue("number")))
	rt.Manager.SetIgnored(number, true)
	writeOK(w, map[string]string{"status": "ignored"})
}

func (h *Handlers) GroupRescrape(w http.ResponseWriter, r *http.Request) {
	rt, _, err := h.getRuntime(r)
	if err != nil {
		writeError(w, 404, err.Error())
		return
	}
	number := strings.ToUpper(strings.TrimSpace(r.PathValue("number")))
	rt.Rescrape(number)
	writeOK(w, map[string]string{"status": "scraping"})
}

func (h *Handlers) ManualTag(w http.ResponseWriter, r *http.Request) {
	rt, _, err := h.getRuntime(r)
	if err != nil {
		writeError(w, 404, err.Error())
		return
	}
	number := strings.ToUpper(strings.TrimSpace(r.PathValue("number")))
	var req struct {
		Path string `json:"path"`
	}
	if err := readJSON(r, &req); err != nil || req.Path == "" {
		writeError(w, 400, "path required")
		return
	}
	rt.Manager.ManualTag(req.Path, number)
	writeOK(w, map[string]string{"status": "tagged"})
}

// ── Unknown Actions ──

func (h *Handlers) UnknownTag(w http.ResponseWriter, r *http.Request) {
	rt, _, err := h.getRuntime(r)
	if err != nil {
		writeError(w, 404, err.Error())
		return
	}
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
	rt.Manager.ManualTag(req.Path, req.Number)
	writeOK(w, map[string]string{"status": "tagged"})
}

func (h *Handlers) UnknownIgnore(w http.ResponseWriter, r *http.Request) {
	rt, _, err := h.getRuntime(r)
	if err != nil {
		writeError(w, 404, err.Error())
		return
	}
	var req struct {
		Path string `json:"path"`
	}
	if err := readJSON(r, &req); err != nil || req.Path == "" {
		writeError(w, 400, "path required")
		return
	}
	rt.Manager.SetUnknownIgnored(req.Path, true)
	writeOK(w, map[string]string{"status": "ignored"})
}

// ── Scan / Link All ──

func (h *Handlers) TriggerScan(w http.ResponseWriter, r *http.Request) {
	rt, _, err := h.getRuntime(r)
	if err != nil {
		writeError(w, 404, err.Error())
		return
	}
	go rt.Scan(context.Background(), h.store)
	writeOK(w, map[string]string{"status": "scanning"})
}

func (h *Handlers) LinkAll(w http.ResponseWriter, r *http.Request) {
	rt, pipeID, err := h.getRuntime(r)
	if err != nil {
		writeError(w, 404, err.Error())
		return
	}

	h.linkAllMu.Lock()
	if h.linkAll == nil {
		h.linkAll = make(map[int64]*LinkAllStatus)
	}
	if s := h.linkAll[pipeID]; s != nil && s.Running {
		h.linkAllMu.Unlock()
		writeOK(w, s)
		return
	}

	groups := rt.Manager.ListGroups()
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

	status := &LinkAllStatus{Total: len(eligible), Running: true}
	h.linkAll[pipeID] = status
	h.linkAllMu.Unlock()

	go func() {
		for i, number := range eligible {
			h.linkAllMu.Lock()
			status.Done = i
			status.Current = number
			h.linkAllMu.Unlock()

			group := rt.Manager.GetGroup(number)
			if group == nil {
				continue
			}
			var paths []string
			for _, item := range group.Items {
				if item.File.Ready {
					paths = append(paths, item.File.Path)
				}
			}
			rt.Manager.SetTask(number, "linking", "")
			if err := rt.Executor.Link(context.Background(), number, paths, rt.LinkOpts()); err != nil {
				log.Printf("[link-all] %s: error: %v", number, err)
				rt.Manager.SetTask(number, "error", err.Error())
				h.linkAllMu.Lock()
				status.Errors = append(status.Errors, fmt.Sprintf("%s: %s", number, err.Error()))
				h.linkAllMu.Unlock()
			}
		}
		h.linkAllMu.Lock()
		status.Done = len(eligible)
		status.Current = ""
		status.Running = false
		h.linkAllMu.Unlock()
	}()

	writeOK(w, status)
}

func (h *Handlers) LinkAllProgress(w http.ResponseWriter, r *http.Request) {
	_, pipeID, err := h.getRuntime(r)
	if err != nil {
		writeError(w, 404, err.Error())
		return
	}
	h.linkAllMu.Lock()
	s := h.linkAll[pipeID]
	h.linkAllMu.Unlock()
	if s == nil {
		writeOK(w, map[string]any{"running": false})
		return
	}
	writeOK(w, s)
}

// ── Library ──

type LibraryItemResponse struct {
	ID           int64    `json:"id"`
	Number       string   `json:"number"`
	SrcPath      string   `json:"srcPath"`
	LinkPath     string   `json:"linkPath"`
	LinkType     string   `json:"linkType"`
	FileSize     int64    `json:"fileSize"`
	Resolution   string   `json:"resolution,omitempty"`
	VideoCodec   string   `json:"videoCodec,omitempty"`
	AudioCodec   string   `json:"audioCodec,omitempty"`
	Duration     string   `json:"duration,omitempty"`
	Bitrate      string   `json:"bitrate,omitempty"`
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
	Premiered    string   `json:"premiered,omitempty"`
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

	sort := r.URL.Query().Get("sort")   // "added", "number", "year", "rating"
	order := r.URL.Query().Get("order") // "asc", "desc"
	outputs, total, err := h.store.ListOutputsByPipeline(ctx, id, size, page*size, sort, order)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}

	metaCache := map[string]*committed.Metadata{}
	items := make([]LibraryItemResponse, 0, len(outputs))
	for _, o := range outputs {
		lv := LibraryItemResponse{
			ID: o.ID, Number: o.Number, SrcPath: o.SrcPath, LinkPath: o.LinkPath,
			LinkType: o.LinkType, FileSize: o.FileSize, Resolution: o.Resolution,
			VideoCodec: o.VideoCodec, AudioCodec: o.AudioCodec, Duration: o.Duration,
			Bitrate: o.Bitrate, Alive: o.Alive,
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
			lv.Premiered = meta.Premiered
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

// ── Library Rescrape ──

func (h *Handlers) LibraryRescrape(w http.ResponseWriter, r *http.Request) {
	number := strings.ToUpper(strings.TrimSpace(r.PathValue("number")))
	if _, loaded := h.libRescrape.LoadOrStore(number, &LibRescrapeResult{Status: "scraping"}); loaded {
		writeError(w, 409, "rescrape already in progress")
		return
	}
	// Use first available runtime for scraping
	runtimes := h.registry.All()
	if len(runtimes) == 0 {
		h.libRescrape.Delete(number)
		writeError(w, 500, "no pipeline running")
		return
	}
	rt := runtimes[0]
	go func() {
		meta, errs := rt.LibScrapeFn(context.Background(), h.store, number)
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
	_ = h.store.UpsertMetadata(r.Context(), &committed.Metadata{
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
	// Find the runtime that owns this output to get the executor
	runtimes := h.registry.All()
	for _, rt := range runtimes {
		if err := rt.Executor.Unlink(r.Context(), number, id); err == nil {
			go rt.Scan(context.Background(), h.store)
			writeOK(w, map[string]string{"status": "unlinked"})
			return
		}
	}
	writeError(w, 500, "unlink failed")
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
			writeError(w, 400, "API ID and Affiliate ID required")
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
