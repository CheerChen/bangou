package web

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"

	"net/http"

	"github.com/CheerChen/bangou/committed"
	"github.com/CheerChen/bangou/config"
	"github.com/CheerChen/bangou/executor"
	"github.com/CheerChen/bangou/provider"
	"github.com/CheerChen/bangou/scanner"
	"github.com/CheerChen/bangou/staging"
)

// Aria2Status provides aria2 connection state to the web layer.
type Aria2Status interface {
	Connected() bool
}

type Handlers struct {
	staging    *staging.Manager
	store      committed.Store
	executor   *executor.Executor
	scanFn         func()
	rescrapeFn     func(string)
	libScrapeFn    func(string) (*provider.MovieMetadata, map[string]string) // standalone scrape for library items
	aria2          Aria2Status
	// Library rescrape: pending new results awaiting user confirmation
	libRescrape sync.Map // number -> *LibRescrapeResult

	// Link All state
	linkAllMu sync.Mutex
	linkAll   *LinkAllStatus
}

type LinkAllStatus struct {
	Total   int
	Done    int
	Current string
	Errors  []string
	Running bool
}

type LibRescrapeResult struct {
	Status string // "scraping", "done", "failed"
	Old    *committed.Metadata
	New    *provider.MovieMetadata
	Errors map[string]string
}

type GroupView struct {
	DOMID        string
	Number       string
	Items        []ItemView
	ParsedCount  int
	TotalSizeGB  float64
	Scrape       staging.ScrapeResult
	MetaTitle    string
	MetaLine     string
	MetaActors   string
	Genres       []string
	SampleImages []string
	Rating       string
	ReviewCount  int
	PageURL      string
	Runtime      string
	ErrorText    string
	Task         string
	TaskErr      string
	TaskProgress int
	AllReady     bool
	Progress     int
	StateClass   string
}

type ItemView struct {
	Path       string
	Filename   string
	Part       int
	SizeGB     float64
	Ready      bool
	Number     string
	SourceSite string
	Tags       string
	Resolution     string
	VideoCodec     string
	AudioCodec     string
	Bitrate        string
	Duration       string
	DownloadPct    int
	DownloadStatus string
}

type UnknownView struct {
	DOMID    string
	Path     string
	Filename string
	SizeGB   float64
}

type LibraryView struct {
	ID              int64
	Number          string
	SrcPath         string
	LinkPath        string
	LinkType        string
	Alive           bool
	Title           string
	MetaLine        string
	Actors          string
	Genres          []string
	CoverURL        string
	SampleImages    []string
	Rating          string
	ReviewCount     int
	PageURL         string
	Runtime         string
	Provider        string
	RescrapeStatus  string
	NewTitle        string
	NewMetaLine     string
	NewActors       string
	NewCoverURL     string
	NewProvider     string
	RescrapeErrors  string
}

type ProviderView struct {
	ID      string
	Name    string
	Enabled bool
}

type DashboardData struct {
	Pending        []GroupView
	Unknown        []UnknownView
	Library        []LibraryView
	LinkableCount  int
	InputDir       string
	Aria2Connected bool
	Aria2Enabled   bool
	Tab            string // "pending" or "library"
	MkvmergePath   string
}

func (h *Handlers) Index(w http.ResponseWriter, r *http.Request) {
	if h.scanFn != nil {
		h.scanFn()
	}
	renderIndex(w, h.buildDashboardData(r.Context()))
}

func (h *Handlers) DashboardPartial(w http.ResponseWriter, r *http.Request) {
	data := h.buildDashboardData(r.Context())
	tab := r.URL.Query().Get("tab")
	if tab == "library" || tab == "pending" {
		data.Tab = tab
	}
	renderPartial(w, "dashboard-content", data)
}

func (h *Handlers) GroupAction(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	number := strings.ToUpper(strings.TrimSpace(r.PathValue("number")))
	action := strings.TrimSpace(r.FormValue("action"))
	if number == "" || action == "" {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	paths := r.Form["paths"]

	switch action {
	case "link":
		if len(paths) == 0 {
			http.Error(w, "no files selected", http.StatusBadRequest)
			return
		}
		h.staging.SetTask(number, "linking", "")
		go func() {
			if err := h.executor.Link(context.Background(), number, paths); err != nil {
				log.Printf("[link] %s: error: %v", number, err)
				h.staging.SetTask(number, "error", err.Error())
			}
		}()
	case "merge":
		if len(paths) < 2 {
			http.Error(w, "select at least 2 files to merge", http.StatusBadRequest)
			return
		}
		h.staging.SetTask(number, "merging", "")
		go func() {
			if err := h.executor.Merge(context.Background(), number, paths); err != nil {
				log.Printf("[merge] %s: error: %v", number, err)
				h.staging.SetTask(number, "error", err.Error())
				return
			}
			h.staging.SetTask(number, "", "")
			// Rescan so the merged file appears in staging
			if h.scanFn != nil {
				h.scanFn()
			}
		}()
	case "ignore":
		h.staging.SetIgnored(number, true)
		fmt.Fprintf(w, `<div id="group-%s"></div>`, domID(number))
		return
	default:
		http.Error(w, "invalid action", http.StatusBadRequest)
		return
	}

	// Return updated card showing task status (merging/linking)
	g := h.staging.GetGroup(number)
	if g == nil {
		fmt.Fprintf(w, `<div id="group-%s"></div>`, domID(number))
		return
	}
	renderPartial(w, "group-card", h.buildGroupView(*g))
}

func (h *Handlers) LinkAll(w http.ResponseWriter, r *http.Request) {
	h.linkAllMu.Lock()
	if h.linkAll != nil && h.linkAll.Running {
		h.linkAllMu.Unlock()
		h.renderLinkAllProgress(w)
		return
	}

	// Collect eligible groups: scrape success, all ready, no active task
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
		fmt.Fprint(w, `<span style="color:#fca5a5;">No eligible groups to link</span>`)
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

	h.renderLinkAllProgress(w)
}

func (h *Handlers) LinkAllProgress(w http.ResponseWriter, r *http.Request) {
	h.renderLinkAllProgress(w)
}

func (h *Handlers) renderLinkAllProgress(w http.ResponseWriter) {
	h.linkAllMu.Lock()
	s := h.linkAll
	h.linkAllMu.Unlock()

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if s == nil {
		fmt.Fprint(w, `<span>No link-all job</span>`)
		return
	}

	if s.Running {
		pct := 0
		if s.Total > 0 {
			pct = s.Done * 100 / s.Total
		}
		fmt.Fprintf(w,
			`<div id="link-all-progress" hx-get="/api/groups/link-all/progress" hx-trigger="every 1s" hx-swap="outerHTML">`+
				`<progress value="%d" max="100" style="margin:0; width:100%%;">%d%%</progress>`+
				`<small>Linking %d/%d: %s</small>`+
				`</div>`,
			pct, pct, s.Done+1, s.Total, s.Current)
		return
	}

	// Done
	errHTML := ""
	if len(s.Errors) > 0 {
		errHTML = fmt.Sprintf(`<small style="color:#fca5a5;">%d errors</small>`, len(s.Errors))
	}
	fmt.Fprintf(w,
		`<div id="link-all-progress" hx-get="/partials/dashboard" hx-trigger="load delay:1s" hx-target="#dashboard-content" hx-swap="outerHTML">`+
			`<small style="color:#6ee7b7;">Linked %d groups</small> %s`+
			`</div>`,
		s.Done-len(s.Errors), errHTML)
}

func (h *Handlers) GroupRescrape(w http.ResponseWriter, r *http.Request) {
	number := strings.ToUpper(strings.TrimSpace(r.PathValue("number")))
	if number == "" {
		http.Error(w, "invalid number", http.StatusBadRequest)
		return
	}
	h.staging.SetScrapeStatus(number, "scraping", map[string]string{})
	if h.rescrapeFn != nil {
		h.rescrapeFn(number)
	}
	g := h.staging.GetGroup(number)
	if g == nil {
		fmt.Fprintf(w, `<div id="group-%s"></div>`, domID(number))
		return
	}
	renderPartial(w, "group-card", h.buildGroupView(*g))
}

func (h *Handlers) TriggerScan(w http.ResponseWriter, r *http.Request) {
	if h.scanFn != nil {
		h.scanFn()
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handlers) ManualTag(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	path := strings.TrimSpace(r.FormValue("path"))
	number := strings.ToUpper(strings.TrimSpace(r.FormValue("number")))
	if path == "" || number == "" {
		http.Error(w, "path and number are required", http.StatusBadRequest)
		return
	}
	h.staging.ManualTag(path, number)
	fmt.Fprintf(w, `<div id="unknown-%s"></div>`, domID(path))
}

func (h *Handlers) IgnoreUnknown(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	path := strings.TrimSpace(r.FormValue("path"))
	if path == "" {
		http.Error(w, "path is required", http.StatusBadRequest)
		return
	}
	h.staging.SetUnknownIgnored(path, true)
	fmt.Fprintf(w, `<div id="unknown-%s"></div>`, domID(path))
}

func (h *Handlers) LibraryRescrape(w http.ResponseWriter, r *http.Request) {
	number := strings.ToUpper(strings.TrimSpace(r.PathValue("number")))
	if number == "" {
		http.Error(w, "invalid number", http.StatusBadRequest)
		return
	}
	h.libRescrape.Store(number, &LibRescrapeResult{Status: "scraping"})
	go func() {
		old, _ := h.store.GetMetadata(context.Background(), number)
		log.Printf("[library] rescraping %s", number)
		meta, errs := h.libScrapeFn(number)
		if meta != nil {
			h.libRescrape.Store(number, &LibRescrapeResult{Status: "done", Old: old, New: meta, Errors: errs})
			log.Printf("[library] rescrape done: %s -> %s (%s)", number, meta.Title, meta.Provider)
		} else {
			h.libRescrape.Store(number, &LibRescrapeResult{Status: "failed", Old: old, Errors: errs})
			log.Printf("[library] rescrape failed: %s", number)
		}
	}()
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handlers) LibraryRescrapeApply(w http.ResponseWriter, r *http.Request) {
	number := strings.ToUpper(strings.TrimSpace(r.PathValue("number")))
	if number == "" {
		http.Error(w, "invalid number", http.StatusBadRequest)
		return
	}
	val, ok := h.libRescrape.Load(number)
	if !ok {
		http.Error(w, "no pending rescrape", http.StatusNotFound)
		return
	}
	result := val.(*LibRescrapeResult)
	if result.New == nil {
		http.Error(w, "no new metadata", http.StatusBadRequest)
		return
	}
	// Update DB with new metadata
	_ = h.store.UpsertMetadata(r.Context(), &committed.Metadata{
		Number:       number,
		Title:        result.New.Title,
		Plot:         result.New.Plot,
		Director:     result.New.Director,
		Maker:        result.New.Maker,
		Label:        result.New.Label,
		Series:       result.New.Series,
		Actors:       strings.Join(result.New.Actors, ","),
		Genres:       strings.Join(result.New.Genres, ","),
		CoverURL:     result.New.CoverURL,
		SampleImages: strings.Join(result.New.SampleImages, ","),
		Premiered:    result.New.Premiered,
		Year:         result.New.Year,
		Runtime:      result.New.Runtime,
		Rating:       result.New.Rating,
		ReviewCount:  result.New.ReviewCount,
		PageURL:      result.New.PageURL,
		ContentID:    result.New.ContentID,
		Provider:     result.New.Provider,
	})
	h.libRescrape.Delete(number)
	log.Printf("[library] rescrape applied: %s", number)
	// Trigger refresh
	w.WriteHeader(http.StatusOK)
}

func (h *Handlers) LibraryRescrapeDismiss(w http.ResponseWriter, r *http.Request) {
	number := strings.ToUpper(strings.TrimSpace(r.PathValue("number")))
	h.libRescrape.Delete(number)
	w.WriteHeader(http.StatusOK)
}

func (h *Handlers) TestAria2(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	url := strings.TrimSpace(r.FormValue("aria2_rpc_url"))
	token := strings.TrimSpace(r.FormValue("aria2_token"))
	if url == "" {
		fmt.Fprint(w, `<span style="color:#fca5a5;">URL is required</span>`)
		return
	}
	ver, err := scanner.TestAria2Connection(url, token)
	if err != nil {
		fmt.Fprintf(w, `<span style="color:#fca5a5;">Failed: %s</span>`, err.Error())
		return
	}
	fmt.Fprintf(w, `<span style="color:#6ee7b7;">Connected! aria2 v%s</span>`, ver)
}

func (h *Handlers) TestDMM(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	apiID := strings.TrimSpace(r.FormValue("dmm_api_id"))
	affID := strings.TrimSpace(r.FormValue("dmm_affiliate_id"))
	if apiID == "" || affID == "" {
		fmt.Fprint(w, `<span style="color:#fca5a5;">API ID and Affiliate ID required</span>`)
		return
	}
	p := provider.NewDMM(apiID, affID)
	_, err := p.Scrape(r.Context(), provider.Predict{Number: "SIVR-476"})
	if err != nil {
		fmt.Fprintf(w, `<span style="color:#fca5a5;">Failed: %s</span>`, err.Error())
		return
	}
	fmt.Fprint(w, `<span style="color:#6ee7b7;">OK! DMM API working</span>`)
}

func (h *Handlers) UnlinkOutput(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimSpace(r.PathValue("id"))
	number := strings.ToUpper(strings.TrimSpace(r.FormValue("number")))
	if idStr == "" || number == "" {
		http.Error(w, "id and number required", http.StatusBadRequest)
		return
	}
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	if err := h.executor.Unlink(r.Context(), number, id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// Trigger rescan so source files re-enter Pending
	if h.scanFn != nil {
		h.scanFn()
	}
	log.Printf("[library] unlinked %s, id=%d", number, id)
	w.WriteHeader(http.StatusOK)
}

func (h *Handlers) DeleteOutput(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimSpace(r.PathValue("id"))
	if idStr == "" {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	if err := h.store.DeleteOutput(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	log.Printf("[library] deleted output id=%d", id)
	w.WriteHeader(http.StatusOK)
}

func (h *Handlers) Settings(w http.ResponseWriter, r *http.Request) {
	renderPartial(w, "settings-modal", h.buildSettingsData(r.Context()))
}

type SettingsData struct {
	Settings  map[string]string
	Providers []ProviderView
}

func (h *Handlers) buildSettingsData(ctx context.Context) SettingsData {
	settings, _ := h.store.GetAllSettings(ctx)
	if settings == nil {
		settings = map[string]string{}
	}
	settings["input_dir"] = config.ResolveSetting(settings["input_dir"], config.EnvInputDir, "/input")
	settings["output_dir"] = config.ResolveSetting(settings["output_dir"], config.EnvOutputDir, "/output")
	settings["aria2_rpc_url"] = config.ResolveSetting(settings["aria2_rpc_url"], config.EnvAria2RPCURL, "")
	settings["aria2_token"] = config.ResolveSetting(settings["aria2_token"], config.EnvAria2Token, "")
	if settings["provider_order"] == "" {
		settings["provider_order"] = "avwiki,dmm"
	}
	if settings["provider_enabled_avwiki"] == "" {
		settings["provider_enabled_avwiki"] = "true"
	}
	if settings["provider_enabled_dmm"] == "" {
		settings["provider_enabled_dmm"] = "true"
	}
	if settings["link_path_pattern"] == "" {
		settings["link_path_pattern"] = "{Number}"
	}

	// Build provider list in order
	order := strings.Split(settings["provider_order"], ",")
	allProviders := map[string]string{"avwiki": "AVWiki", "dmm": "DMM"}
	var providers []ProviderView
	seen := map[string]bool{}
	for _, id := range order {
		id = strings.TrimSpace(strings.ToLower(id))
		if name, ok := allProviders[id]; ok {
			providers = append(providers, ProviderView{
				ID:      id,
				Name:    name,
				Enabled: settings["provider_enabled_"+id] != "false",
			})
			seen[id] = true
		}
	}
	// Add any missing providers at the end
	for id, name := range allProviders {
		if !seen[id] {
			providers = append(providers, ProviderView{
				ID:      id,
				Name:    name,
				Enabled: settings["provider_enabled_"+id] != "false",
			})
		}
	}

	return SettingsData{Settings: settings, Providers: providers}
}

func (h *Handlers) SaveSettings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	_ = r.ParseForm()
	keys := []string{
		"input_dir", "output_dir", "aria2_rpc_url", "aria2_token",
		"provider_order", "dmm_api_id", "dmm_affiliate_id",
		"link_path_pattern",
	}
	for _, key := range keys {
		if err := h.store.SetSetting(ctx, key, strings.TrimSpace(r.FormValue(key))); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	avwikiEnabled := "false"
	if r.FormValue("provider_enabled_avwiki") != "" {
		avwikiEnabled = "true"
	}
	dmmEnabled := "false"
	if r.FormValue("provider_enabled_dmm") != "" {
		dmmEnabled = "true"
	}
	if err := h.store.SetSetting(ctx, "provider_enabled_avwiki", avwikiEnabled); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := h.store.SetSetting(ctx, "provider_enabled_dmm", dmmEnabled); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	log.Printf("[settings] saved")
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handlers) buildDashboardData(ctx context.Context) DashboardData {
	groups := h.staging.ListGroups()
	unknowns := h.staging.ListUnknowns()
	outputs, _ := h.store.ListAllOutputs(ctx)
	settings, _ := h.store.GetAllSettings(ctx)
	inputDir := config.ResolveSetting(settings["input_dir"], config.EnvInputDir, "/input")

	pending := make([]GroupView, 0, len(groups))
	for _, g := range groups {
		pending = append(pending, h.buildGroupView(g))
	}
	unk := make([]UnknownView, 0, len(unknowns))
	for _, u := range unknowns {
		unk = append(unk, UnknownView{
			DOMID:    domID(u.Path),
			Path:     u.Path,
			Filename: u.Filename,
			SizeGB:   float64(u.Size) / (1024.0 * 1024 * 1024),
		})
	}
	lib := make([]LibraryView, 0, len(outputs))
	metaCache := map[string]*committed.Metadata{}
	for _, o := range outputs {
		lv := LibraryView{ID: o.ID, Number: o.Number, SrcPath: o.SrcPath, LinkPath: o.LinkPath, LinkType: o.LinkType, Alive: o.Alive}
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
			lv.Runtime = meta.Runtime
			lv.Genres = splitCSV(meta.Genres)
			lv.SampleImages = splitCSV(meta.SampleImages)
			parts := []string{}
			if meta.Premiered != "" {
				parts = append(parts, meta.Premiered)
			}
			if meta.Maker != "" {
				parts = append(parts, meta.Maker)
			}
			if meta.Label != "" {
				parts = append(parts, meta.Label)
			}
			if meta.Runtime != "" {
				parts = append(parts, meta.Runtime+"min")
			}
			lv.MetaLine = strings.Join(parts, " | ")
		}
		// Check for pending rescrape
		if val, ok := h.libRescrape.Load(o.Number); ok {
			rs := val.(*LibRescrapeResult)
			lv.RescrapeStatus = rs.Status
			if rs.New != nil {
				lv.NewTitle = rs.New.Title
				lv.NewCoverURL = rs.New.CoverURL
				lv.NewProvider = rs.New.Provider
				lv.NewActors = strings.Join(rs.New.Actors, ", ")
				np := []string{}
				if rs.New.Premiered != "" {
					np = append(np, rs.New.Premiered)
				}
				if rs.New.Maker != "" {
					np = append(np, rs.New.Maker)
				}
				if rs.New.Label != "" {
					np = append(np, rs.New.Label)
				}
				lv.NewMetaLine = strings.Join(np, " | ")
			}
			if rs.Errors != nil {
				ep := make([]string, 0, len(rs.Errors))
				for k, v := range rs.Errors {
					ep = append(ep, k+": "+v)
				}
				lv.RescrapeErrors = strings.Join(ep, "; ")
			}
		}
		lib = append(lib, lv)
	}

	aria2Enabled := h.aria2 != nil
	aria2Connected := aria2Enabled && h.aria2.Connected()

	mkvmergePath := ""
	if p, err := exec.LookPath("mkvmerge"); err == nil {
		mkvmergePath = p
	}

	linkable := 0
	for _, gv := range pending {
		if gv.Scrape.Status == "success" && gv.AllReady && gv.Task == "" {
			linkable++
		}
	}

	return DashboardData{Pending: pending, Unknown: unk, Library: lib, LinkableCount: linkable, InputDir: inputDir, Aria2Enabled: aria2Enabled, Aria2Connected: aria2Connected, MkvmergePath: mkvmergePath}
}

func (h *Handlers) buildGroupView(g staging.StagingGroup) GroupView {
	gv := GroupView{DOMID: domID(g.Number), Number: g.Number, Scrape: g.Scrape, Task: g.Task, TaskErr: g.TaskErr, TaskProgress: g.TaskProgress}
	var totalSize int64
	for _, item := range g.Items {
		totalSize += item.File.Size
		iv := ItemView{
			Path:       item.File.Path,
			Filename:   item.File.Filename,
			Part:       item.Parsed.Part,
			SizeGB:     float64(item.File.Size) / (1024.0 * 1024 * 1024),
			Ready:      item.File.Ready,
			Number:     item.Parsed.Number,
			SourceSite: item.Parsed.SourceSite,
			Tags:       strings.Join(item.Parsed.Tags, ","),
		}
		if m := item.File.Media; m != nil {
			iv.Resolution = m.Resolution()
			iv.VideoCodec = m.VideoCodec
			iv.AudioCodec = m.AudioCodec
			if m.AudioChannels > 0 {
				iv.AudioCodec = fmt.Sprintf("%s %dch", m.AudioCodec, m.AudioChannels)
			}
			iv.Bitrate = m.BitrateText()
			iv.Duration = m.DurationText()
		}
		if !item.File.Ready {
			iv.DownloadPct = item.File.DownloadPct
			iv.DownloadStatus = item.File.DownloadStatus
		}
		gv.Items = append(gv.Items, iv)
	}
	gv.ParsedCount = len(g.Items)
	gv.TotalSizeGB = float64(totalSize) / (1024.0 * 1024 * 1024)
	gv.AllReady = true
	for _, item := range g.Items {
		if !item.File.Ready {
			gv.AllReady = false
			break
		}
	}

	// Unified progress for card background
	if g.Task == "merging" && g.TaskProgress > 0 {
		gv.Progress = g.TaskProgress
	} else if !gv.AllReady {
		// Average download progress across not-ready files
		var total, count int
		for _, item := range g.Items {
			if !item.File.Ready {
				total += item.File.DownloadPct
				count++
			}
		}
		if count > 0 {
			gv.Progress = total / count
		}
	}

	// State class for left border color
	switch {
	case g.Task == "merging":
		gv.StateClass = "state-merging"
	case g.Task == "linking":
		gv.StateClass = "state-merging"
	case g.Task == "error":
		gv.StateClass = "state-error"
	case !gv.AllReady:
		// Check if any file is paused
		paused := false
		for _, item := range g.Items {
			if item.File.DownloadStatus == "paused" {
				paused = true
				break
			}
		}
		if paused {
			gv.StateClass = "state-paused"
		} else {
			gv.StateClass = "state-downloading"
		}
	case g.Scrape.Status == "scraping":
		gv.StateClass = "state-scraping"
	case g.Scrape.Status == "failed":
		gv.StateClass = "state-error"
	case g.Scrape.Status == "success":
		gv.StateClass = "state-ready"
	default:
		gv.StateClass = "state-scraping"
	}

	if g.Scrape.Meta != nil {
		m := g.Scrape.Meta
		gv.MetaTitle = m.Title
		parts := []string{}
		if m.Premiered != "" {
			parts = append(parts, m.Premiered)
		}
		if m.Maker != "" {
			parts = append(parts, m.Maker)
		}
		if m.Label != "" {
			parts = append(parts, m.Label)
		}
		if m.Runtime != "" {
			parts = append(parts, m.Runtime+"min")
		}
		gv.MetaLine = strings.Join(parts, " | ")
		gv.MetaActors = strings.Join(m.Actors, ", ")
		gv.Genres = m.Genres
		gv.SampleImages = m.SampleImages
		gv.Rating = m.Rating
		gv.ReviewCount = m.ReviewCount
		gv.PageURL = m.PageURL
		gv.Runtime = m.Runtime
	}
	if len(g.Scrape.Errors) > 0 {
		keys := make([]string, 0, len(g.Scrape.Errors))
		for k := range g.Scrape.Errors {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		parts := make([]string, 0, len(keys))
		for _, k := range keys {
			parts = append(parts, k+": "+g.Scrape.Errors[k])
		}
		gv.ErrorText = strings.Join(parts, "; ")
	}
	return gv
}

var idSanitizer = regexp.MustCompile(`[^a-zA-Z0-9_-]+`)

func domID(s string) string {
	out := idSanitizer.ReplaceAllString(strings.TrimSpace(s), "_")
	if out == "" {
		return "x"
	}
	return out
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
