package nfo

import (
	"encoding/xml"
	"fmt"
	"os"

	"github.com/CheerChen/bangou/provider"
)

type movie struct {
	XMLName   xml.Name `xml:"movie"`
	Plot      string   `xml:"plot"`
	Title     string   `xml:"title"`
	Director  string   `xml:"director,omitempty"`
	Year      string   `xml:"year,omitempty"`
	Premiered string   `xml:"premiered,omitempty"`
	Runtime   string   `xml:"runtime,omitempty"`
	Genre     []string `xml:"genre"`
	Studio    string   `xml:"studio,omitempty"`
	Set       *set     `xml:"set,omitempty"`
	Tag       []string `xml:"tag"`
	Actor     []actor  `xml:"actor"`
	Label     string   `xml:"label,omitempty"`
	Num       string   `xml:"num"`
	Cover     string   `xml:"cover,omitempty"`
}

type set struct {
	Name string `xml:"name"`
}

type actor struct {
	Name string `xml:"name"`
}

func Generate(meta *provider.MovieMetadata) ([]byte, error) {
	if meta == nil {
		return nil, fmt.Errorf("nil metadata")
	}
	actors := make([]actor, 0, len(meta.Actors))
	for _, a := range meta.Actors {
		actors = append(actors, actor{Name: a})
	}
	tags := append([]string{}, meta.Genres...)
	for _, t := range []string{meta.Maker, meta.Label, meta.Series, meta.Director} {
		if t != "" {
			tags = append(tags, t)
		}
	}

	var s *set
	if meta.Series != "" {
		s = &set{Name: meta.Series}
	}

	m := movie{
		Plot:      meta.Plot,
		Title:     fmt.Sprintf("%s %s", meta.Number, meta.Title),
		Director:  meta.Director,
		Year:      meta.Year,
		Premiered: meta.Premiered,
		Runtime:   meta.Runtime,
		Genre:     meta.Genres,
		Studio:    meta.Maker,
		Set:       s,
		Tag:       tags,
		Actor:     actors,
		Label:     meta.Label,
		Num:       meta.Number,
		Cover:     meta.CoverURL,
	}

	x, err := xml.MarshalIndent(m, "", "  ")
	if err != nil {
		return nil, err
	}
	return []byte(xml.Header + string(x)), nil
}

func Save(meta *provider.MovieMetadata, path string) error {
	data, err := Generate(meta)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
