package parser

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

type ParsedFile struct {
	Number     string
	Part       int
	Tags       []string
	Ext        string
	SourceSite string
}

var (
	sitePrefixRe = regexp.MustCompile(`^([a-zA-Z0-9.-]+)@`)
	partRe       = regexp.MustCompile(`_(\d+)_`)
	tagRe        = regexp.MustCompile(`(?i)(?:^|[_\-\s])(8k|4k|vr)(?:$|[_\-\s.])`)
	mgstageRe    = regexp.MustCompile(`(?i)^(\d{3,4}[a-zA-Z]{2,6})-?(\d{3,4})\b`)
	heyzoRe      = regexp.MustCompile(`(?i)^(heyzo)-?(\d{4})\b`)
	standardRe   = regexp.MustCompile(`(?i)^([a-zA-Z]{2,5})-?(\d{3,6})\b`)
)

func Parse(filename string) ParsedFile {
	ext := strings.ToLower(filepath.Ext(filename))
	name := strings.TrimSuffix(filename, filepath.Ext(filename))

	res := ParsedFile{Ext: ext}

	if m := sitePrefixRe.FindStringSubmatch(name); len(m) > 1 {
		res.SourceSite = strings.ToLower(m[1])
		name = sitePrefixRe.ReplaceAllString(name, "")
	}

	if m := partRe.FindStringSubmatch(name); len(m) > 1 {
		if p, err := strconv.Atoi(m[1]); err == nil {
			res.Part = p
		}
		name = partRe.ReplaceAllString(name, "_")
	}

	for _, m := range tagRe.FindAllStringSubmatch(name, -1) {
		if len(m) > 1 {
			res.Tags = append(res.Tags, strings.ToLower(m[1]))
		}
	}
	res.Tags = unique(res.Tags)
	res.Number = extractNumber(strings.ReplaceAll(name, "_", "-"))

	return res
}

func extractNumber(name string) string {
	if m := heyzoRe.FindStringSubmatch(name); len(m) > 2 {
		return strings.ToUpper(m[1]) + "-" + m[2]
	}
	if m := mgstageRe.FindStringSubmatch(name); len(m) > 2 {
		return strings.ToUpper(m[1]) + "-" + trimLeadingZeros(m[2])
	}
	if m := standardRe.FindStringSubmatch(name); len(m) > 2 {
		return strings.ToUpper(m[1]) + "-" + trimLeadingZeros(m[2])
	}
	return ""
}

func trimLeadingZeros(s string) string {
	n, err := strconv.Atoi(s)
	if err != nil {
		return s
	}
	out := strconv.Itoa(n)
	for len(out) < 3 {
		out = "0" + out
	}
	return out
}

func unique(tags []string) []string {
	if len(tags) < 2 {
		return tags
	}
	seen := make(map[string]struct{}, len(tags))
	out := make([]string, 0, len(tags))
	for _, t := range tags {
		if _, ok := seen[t]; ok {
			continue
		}
		seen[t] = struct{}{}
		out = append(out, t)
	}
	return out
}
