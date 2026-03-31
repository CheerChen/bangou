package web

import (
	"embed"
	"html/template"
	"io/fs"
	"net/http"

	"github.com/CheerChen/bangou/committed"
	"github.com/CheerChen/bangou/executor"
	"github.com/CheerChen/bangou/provider"
	"github.com/CheerChen/bangou/staging"
)

//go:embed templates/*.html static/*
var content embed.FS

var (
	indexTemplates    = template.Must(template.ParseFS(content, "templates/layout.html", "templates/group.html", "templates/index.html"))
	settingsTemplates = template.Must(template.ParseFS(content, "templates/layout.html", "templates/settings.html"))
	partialTemplates  = template.Must(template.ParseFS(content, "templates/group.html", "templates/index.html", "templates/settings.html"))
	staticFS          = mustSubFS(content, "static")
)

func renderIndex(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = indexTemplates.ExecuteTemplate(w, "layout", data)
}

func renderSettings(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = settingsTemplates.ExecuteTemplate(w, "layout", data)
}

func renderPartial(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = partialTemplates.ExecuteTemplate(w, name, data)
}

func mustSubFS(root fs.FS, dir string) fs.FS {
	sub, err := fs.Sub(root, dir)
	if err != nil {
		panic(err)
	}
	return sub
}

func NewServer(stg *staging.Manager, store committed.Store, exec *executor.Executor, scanFn func(), rescrapeFn func(string), libScrapeFn func(string) (*provider.MovieMetadata, map[string]string), aria2 Aria2Status) http.Handler {
	h := &Handlers{staging: stg, store: store, executor: exec, scanFn: scanFn, rescrapeFn: rescrapeFn, libScrapeFn: libScrapeFn, aria2: aria2}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", h.Index)
	mux.HandleFunc("GET /partials/dashboard", h.DashboardPartial)
	mux.HandleFunc("GET /settings", h.Settings)
	mux.HandleFunc("POST /api/settings", h.SaveSettings)
	mux.HandleFunc("POST /api/groups/link-all", h.LinkAll)
	mux.HandleFunc("GET /api/groups/link-all/progress", h.LinkAllProgress)
	mux.HandleFunc("POST /api/groups/{number}/action", h.GroupAction)
	mux.HandleFunc("POST /api/groups/{number}/rescrape", h.GroupRescrape)
	mux.HandleFunc("POST /api/scan", h.TriggerScan)
	mux.HandleFunc("POST /api/test/aria2", h.TestAria2)
	mux.HandleFunc("POST /api/test/dmm", h.TestDMM)
	mux.HandleFunc("POST /api/files/tag", h.ManualTag)
	mux.HandleFunc("POST /api/files/ignore", h.IgnoreUnknown)
	mux.HandleFunc("POST /api/library/{number}/rescrape", h.LibraryRescrape)
	mux.HandleFunc("POST /api/library/{number}/apply", h.LibraryRescrapeApply)
	mux.HandleFunc("POST /api/library/{number}/dismiss", h.LibraryRescrapeDismiss)
	mux.HandleFunc("POST /api/outputs/{id}/unlink", h.UnlinkOutput)
	mux.HandleFunc("DELETE /api/outputs/{id}", h.DeleteOutput)
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServerFS(staticFS)))
	return mux
}
