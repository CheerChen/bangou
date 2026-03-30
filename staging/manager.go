package staging

import (
	"sort"
	"strings"
	"sync"

	"github.com/CheerChen/bangou/parser"
)

type Manager struct {
	mu        sync.RWMutex
	groups    map[string]*StagingGroup
	unknowns  map[string]*UnknownFile
	fileIndex map[string]string

	OnNewNumber func(number string)
}

func New() *Manager {
	return &Manager{
		groups:    map[string]*StagingGroup{},
		unknowns:  map[string]*UnknownFile{},
		fileIndex: map[string]string{},
	}
}

func (m *Manager) Ingest(f StagingFile) {
	parsed := parser.Parse(f.Filename)
	trigger := ""

	m.mu.Lock()
	if parsed.Number == "" {
		u, ok := m.unknowns[f.Path]
		if ok {
			if f.Media == nil {
				f.Media = u.Media
			}
			u.StagingFile = f
		} else {
			m.unknowns[f.Path] = &UnknownFile{StagingFile: f}
		}
		m.mu.Unlock()
		return
	}

	delete(m.unknowns, f.Path)
	g, exists := m.groups[parsed.Number]
	if !exists {
		g = &StagingGroup{Number: parsed.Number}
		m.groups[parsed.Number] = g
		trigger = parsed.Number
	}

	for i := range g.Items {
		if g.Items[i].File.Path == f.Path {
			if f.Media == nil {
				f.Media = g.Items[i].File.Media
			}
			g.Items[i].File.Size = f.Size
			g.Items[i].File.Ready = f.Ready
			g.Items[i].File.Filename = f.Filename
			g.Items[i].File.Media = f.Media
			m.mu.Unlock()
			return
		}
	}

	g.Items = append(g.Items, StagedItem{
		File: f,
		Parsed: ParsedFile{
			Number:     parsed.Number,
			Part:       parsed.Part,
			Tags:       append([]string(nil), parsed.Tags...),
			SourceSite: parsed.SourceSite,
		},
	})
	m.fileIndex[f.Path] = parsed.Number
	m.mu.Unlock()

	if trigger != "" && m.OnNewNumber != nil {
		go m.OnNewNumber(trigger)
	}
}

// HasMedia returns true if the file at path already has media info cached and size matches.
func (m *Manager) HasMedia(path string, size int64) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if u, ok := m.unknowns[path]; ok {
		return u.Media != nil && u.Size == size
	}
	number, ok := m.fileIndex[path]
	if !ok {
		return false
	}
	g, ok := m.groups[number]
	if !ok {
		return false
	}
	for _, item := range g.Items {
		if item.File.Path == path {
			return item.File.Media != nil && item.File.Size == size
		}
	}
	return false
}

func (m *Manager) Remove(path string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.unknowns, path)
	number, ok := m.fileIndex[path]
	if !ok {
		return
	}
	delete(m.fileIndex, path)

	g, ok := m.groups[number]
	if !ok {
		return
	}
	alive := g.Items[:0]
	for _, item := range g.Items {
		if item.File.Path == path {
			continue
		}
		alive = append(alive, item)
	}
	g.Items = alive
	if len(g.Items) == 0 {
		delete(m.groups, number)
	}
}

func (m *Manager) Reconcile(existingPaths map[string]bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for number, g := range m.groups {
		alive := g.Items[:0]
		for _, item := range g.Items {
			if existingPaths[item.File.Path] {
				alive = append(alive, item)
			} else {
				delete(m.fileIndex, item.File.Path)
			}
		}
		g.Items = alive
		if len(g.Items) == 0 {
			delete(m.groups, number)
		}
	}
	for path := range m.unknowns {
		if !existingPaths[path] {
			delete(m.unknowns, path)
		}
	}
}

func (m *Manager) RemoveGroup(number string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	g, ok := m.groups[number]
	if !ok {
		return
	}
	for _, item := range g.Items {
		delete(m.fileIndex, item.File.Path)
	}
	delete(m.groups, number)
}

func (m *Manager) GetGroup(number string) *StagingGroup {
	m.mu.RLock()
	defer m.mu.RUnlock()
	g := m.groups[number]
	if g == nil {
		return nil
	}
	return copyGroup(g)
}

func (m *Manager) ListGroups() []StagingGroup {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make([]StagingGroup, 0, len(m.groups))
	for _, g := range m.groups {
		if g.Ignored {
			continue
		}
		out = append(out, *copyGroup(g))
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Number < out[j].Number
	})
	return out
}

func (m *Manager) ListUnknowns() []UnknownFile {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make([]UnknownFile, 0, len(m.unknowns))
	for _, u := range m.unknowns {
		if u.Ignored {
			continue
		}
		out = append(out, *u)
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].Filename) < strings.ToLower(out[j].Filename)
	})
	return out
}

func (m *Manager) SetScrapeResult(number string, result ScrapeResult) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if g, ok := m.groups[number]; ok {
		if result.Errors == nil {
			result.Errors = map[string]string{}
		}
		g.Scrape = result
	}
}

func (m *Manager) SetScrapeStatus(number, status string, errs map[string]string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if g, ok := m.groups[number]; ok {
		if errs != nil {
			g.Scrape.Errors = errs
		}
		g.Scrape.Status = status
		if status != "success" {
			g.Scrape.Meta = nil
		}
	}
}

func (m *Manager) SetTask(number, task, taskErr string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if g, ok := m.groups[number]; ok {
		g.Task = task
		g.TaskErr = taskErr
		g.TaskProgress = 0
	}
}

func (m *Manager) SetTaskProgress(number string, percent int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if g, ok := m.groups[number]; ok {
		g.TaskProgress = percent
	}
}

// SetDownloadProgress updates download progress for a file matched by filename.
func (m *Manager) SetDownloadProgress(filename string, pct int, completed int64, status string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, u := range m.unknowns {
		if u.Filename == filename {
			u.DownloadPct = pct
			u.DownloadSize = completed
			u.DownloadStatus = status
			return
		}
	}
	for _, g := range m.groups {
		for i := range g.Items {
			if g.Items[i].File.Filename == filename {
				g.Items[i].File.DownloadPct = pct
				g.Items[i].File.DownloadSize = completed
				g.Items[i].File.DownloadStatus = status
				return
			}
		}
	}
}

// ClearDownloadProgress resets download info for files not in the given filename set.
func (m *Manager) ClearDownloadProgress(activeFilenames map[string]bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, u := range m.unknowns {
		if u.DownloadStatus != "" && !activeFilenames[u.Filename] {
			u.DownloadPct = 0
			u.DownloadSize = 0
			u.DownloadStatus = ""
		}
	}
	for _, g := range m.groups {
		for i := range g.Items {
			f := &g.Items[i].File
			if f.DownloadStatus != "" && !activeFilenames[f.Filename] {
				f.DownloadPct = 0
				f.DownloadSize = 0
				f.DownloadStatus = ""
			}
		}
	}
}

func (m *Manager) SetIgnored(number string, ignored bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if g, ok := m.groups[number]; ok {
		g.Ignored = ignored
	}
}

func (m *Manager) SetUnknownIgnored(path string, ignored bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if u, ok := m.unknowns[path]; ok {
		u.Ignored = ignored
	}
}

func (m *Manager) ManualTag(path string, number string) {
	number = strings.ToUpper(strings.TrimSpace(number))
	if number == "" {
		return
	}
	var trigger string

	m.mu.Lock()
	u, ok := m.unknowns[path]
	if !ok {
		m.mu.Unlock()
		return
	}
	delete(m.unknowns, path)

	g, exists := m.groups[number]
	if !exists {
		g = &StagingGroup{Number: number}
		m.groups[number] = g
		trigger = number
	}
	g.Items = append(g.Items, StagedItem{
		File: u.StagingFile,
		Parsed: ParsedFile{
			Number: number,
		},
	})
	m.fileIndex[path] = number
	m.mu.Unlock()

	if trigger != "" && m.OnNewNumber != nil {
		go m.OnNewNumber(trigger)
	}
}

func copyGroup(g *StagingGroup) *StagingGroup {
	cpy := *g
	cpy.Items = make([]StagedItem, len(g.Items))
	for i := range g.Items {
		cpy.Items[i] = g.Items[i]
		cpy.Items[i].Parsed.Tags = append([]string(nil), g.Items[i].Parsed.Tags...)
	}
	if g.Scrape.Errors != nil {
		cpy.Scrape.Errors = make(map[string]string, len(g.Scrape.Errors))
		for k, v := range g.Scrape.Errors {
			cpy.Scrape.Errors[k] = v
		}
	}
	if g.Scrape.Meta != nil {
		meta := *g.Scrape.Meta
		meta.Actors = append([]string(nil), g.Scrape.Meta.Actors...)
		meta.Genres = append([]string(nil), g.Scrape.Meta.Genres...)
		cpy.Scrape.Meta = &meta
	}
	return &cpy
}
