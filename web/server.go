package web

import (
	"encoding/json"
	"net/http"

	"github.com/CheerChen/bangou/committed"
)

func NewServer(reg *Registry, store committed.Store) http.Handler {
	h := &Handlers{registry: reg, store: store}
	mux := http.NewServeMux()

	// Pipelines
	mux.HandleFunc("GET /api/pipelines", h.ListPipelines)
	mux.HandleFunc("POST /api/pipelines", h.CreatePipeline)
	mux.HandleFunc("DELETE /api/pipelines/{id}", h.DeletePipeline)

	// Pipeline-scoped
	mux.HandleFunc("GET /api/pipelines/{id}/groups", h.ListGroups)
	mux.HandleFunc("GET /api/pipelines/{id}/library", h.ListLibrary)
	mux.HandleFunc("POST /api/pipelines/{id}/scan", h.TriggerScan)
	mux.HandleFunc("POST /api/pipelines/{id}/link-all", h.LinkAll)
	mux.HandleFunc("GET /api/pipelines/{id}/link-all/progress", h.LinkAllProgress)

	// Group actions (pipeline resolved from group's number)
	mux.HandleFunc("POST /api/pipelines/{id}/groups/{number}/link", h.GroupLink)
	mux.HandleFunc("POST /api/pipelines/{id}/groups/{number}/merge", h.GroupMerge)
	mux.HandleFunc("POST /api/pipelines/{id}/groups/{number}/ignore", h.GroupIgnore)
	mux.HandleFunc("POST /api/pipelines/{id}/groups/{number}/rescrape", h.GroupRescrape)
	mux.HandleFunc("POST /api/pipelines/{id}/groups/{number}/tag", h.ManualTag)

	// Unknown file actions
	mux.HandleFunc("POST /api/pipelines/{id}/unknowns/tag", h.UnknownTag)
	mux.HandleFunc("POST /api/pipelines/{id}/unknowns/ignore", h.UnknownIgnore)

	// Library actions
	mux.HandleFunc("POST /api/library/{number}/rescrape", h.LibraryRescrape)
	mux.HandleFunc("POST /api/library/{number}/apply", h.LibraryRescrapeApply)
	mux.HandleFunc("POST /api/library/{number}/dismiss", h.LibraryRescrapeDismiss)
	mux.HandleFunc("POST /api/outputs/{id}/unlink", h.UnlinkOutput)
	mux.HandleFunc("DELETE /api/outputs/{id}", h.DeleteOutput)

	// Provider configs
	mux.HandleFunc("GET /api/provider-configs", h.ListProviderConfigs)
	mux.HandleFunc("PUT /api/provider-configs/{provider}", h.SetProviderConfig)
	mux.HandleFunc("POST /api/provider-configs/{provider}/test", h.TestProviderConfig)

	return mux
}

// JSON helpers

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeOK(w http.ResponseWriter, v any) {
	writeJSON(w, http.StatusOK, v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func readJSON(r *http.Request, v any) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(v)
}

func encodeJSON(v any) (string, error) {
	b, err := json.Marshal(v)
	return string(b), err
}

func decodeJSON(s string, v any) error {
	return json.Unmarshal([]byte(s), v)
}
