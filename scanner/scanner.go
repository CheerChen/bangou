package scanner

import (
	"os"
	"path/filepath"
	"strings"
)

var videoExts = map[string]bool{
	".mp4": true,
	".mkv": true,
	".avi": true,
	".wmv": true,
}

type ScannedFile struct {
	Path     string
	Filename string
	Size     int64
	Ready    bool
}

// ScanDir runs a single directory pass and returns detected video files.
func ScanDir(dir string) ([]ScannedFile, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	aria2 := make(map[string]bool, len(entries))
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".aria2") {
			aria2[strings.TrimSuffix(e.Name(), ".aria2")] = true
		}
	}

	out := make([]ScannedFile, 0)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if !videoExts[ext] {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		out = append(out, ScannedFile{
			Path:     filepath.Join(dir, e.Name()),
			Filename: e.Name(),
			Size:     info.Size(),
			Ready:    !aria2[e.Name()],
		})
	}
	return out, nil
}

