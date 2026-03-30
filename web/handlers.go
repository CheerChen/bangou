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

	"net/http"

	"github.com/CheerChen/bangou/committed"
	"github.com/CheerChen/bangou/config"
	"github.com/CheerChen/bangou/executor"
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
	scanFn     func()
	rescrapeFn func(string)
	aria2      Aria2Status
}

type GroupView struct {
	DOMID       string
	Number      string
	Items       []ItemView
	ParsedCount int
	TotalSizeGB float64
	TagText     string
	SourceText  string
	Scrape      staging.ScrapeResult
	MetaTitle   string
	MetaLine    string
	MetaActors  string
	ErrorText    string
	Task         string
	TaskErr      string
	TaskProgress int
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
	Resolution string
	VideoCodec string
	AudioCodec string
	Bitrate    string
	Duration   string
}

type UnknownView struct {
	DOMID    string
	Path     string
	Filename string
	SizeGB   float64
}

type LibraryView struct {
	ID       int64
	Number   string
	LinkPath string
	LinkType string
	Alive    bool
}

type DashboardData struct {
	Pending        []GroupView
	Unknown        []UnknownView
	Library        []LibraryView
	InputDir       string
	Aria2Connected bool
	Aria2Enabled   bool
}

func (h *Handlers) Index(w http.ResponseWriter, r *http.Request) {
	if h.scanFn != nil {
		h.scanFn()
	}
	renderIndex(w, h.buildDashboardData(r.Context()))
}

func (h *Handlers) DashboardPartial(w http.ResponseWriter, r *http.Request) {
	renderPartial(w, "dashboard-content", h.buildDashboardData(r.Context()))
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
	ctx := r.Context()
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
	if p, err := exec.LookPath("mkvmerge"); err == nil {
		settings["mkvmerge_available"] = "1"
		settings["mkvmerge_path"] = p
	}
	renderSettings(w, settings)
}

func (h *Handlers) SaveSettings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	_ = r.ParseForm()
	keys := []string{
		"input_dir", "output_dir", "aria2_rpc_url", "aria2_token",
		"provider_order", "dmm_api_id", "dmm_affiliate_id",
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
	http.Redirect(w, r, "/settings", http.StatusSeeOther)
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
	for _, o := range outputs {
		lib = append(lib, LibraryView{ID: o.ID, Number: o.Number, LinkPath: o.LinkPath, LinkType: o.LinkType, Alive: o.Alive})
	}

	aria2Enabled := h.aria2 != nil
	aria2Connected := aria2Enabled && h.aria2.Connected()

	return DashboardData{Pending: pending, Unknown: unk, Library: lib, InputDir: inputDir, Aria2Enabled: aria2Enabled, Aria2Connected: aria2Connected}
}

func (h *Handlers) buildGroupView(g staging.StagingGroup) GroupView {
	gv := GroupView{DOMID: domID(g.Number), Number: g.Number, Scrape: g.Scrape, Task: g.Task, TaskErr: g.TaskErr, TaskProgress: g.TaskProgress}
	tagSet := map[string]struct{}{}
	sourceSet := map[string]struct{}{}
	var totalSize int64
	for _, item := range g.Items {
		totalSize += item.File.Size
		for _, t := range item.Parsed.Tags {
			tagSet[t] = struct{}{}
		}
		if item.Parsed.SourceSite != "" {
			sourceSet[item.Parsed.SourceSite] = struct{}{}
		}
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
		gv.Items = append(gv.Items, iv)
	}
	gv.ParsedCount = len(g.Items)
	gv.TotalSizeGB = float64(totalSize) / (1024.0 * 1024 * 1024)
	tags := make([]string, 0, len(tagSet))
	for t := range tagSet {
		tags = append(tags, t)
	}
	sort.Strings(tags)
	gv.TagText = strings.Join(tags, ", ")
	sources := make([]string, 0, len(sourceSet))
	for s := range sourceSet {
		sources = append(sources, s)
	}
	sort.Strings(sources)
	gv.SourceText = strings.Join(sources, ", ")

	if g.Scrape.Meta != nil {
		gv.MetaTitle = g.Scrape.Meta.Title
		parts := []string{}
		if g.Scrape.Meta.Premiered != "" {
			parts = append(parts, g.Scrape.Meta.Premiered)
		}
		if g.Scrape.Meta.Maker != "" {
			parts = append(parts, g.Scrape.Meta.Maker)
		}
		if g.Scrape.Meta.Label != "" {
			parts = append(parts, g.Scrape.Meta.Label)
		}
		gv.MetaLine = strings.Join(parts, " | ")
		gv.MetaActors = strings.Join(g.Scrape.Meta.Actors, ", ")
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
