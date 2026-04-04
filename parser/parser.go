package parser

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

type ParsedFile struct {
	Number     string
	RawNumber  string
	Part       int
	Tags       []string
	Ext        string
	SourceSite string
}

var (
	bracketSiteRe = regexp.MustCompile(`^\[([a-zA-Z0-9.-]+)\]`)
	sitePrefixRe  = regexp.MustCompile(`^([a-zA-Z0-9.-]+)@`)
	tokenizeRe   = regexp.MustCompile(`[^a-zA-Z0-9]+`)
	partTokenRe  = regexp.MustCompile(`(?i)^part(\d+)$`)
	tagTokenRe   = regexp.MustCompile(`(?i)^(8k|4k|vr)$`)

	heyzoRe    = regexp.MustCompile(`(?i)^(heyzo)(\d{4})(?:\D|$)`)
	mgstageRe  = regexp.MustCompile(`(?i)^(\d{3,4}[a-zA-Z]{2,6})(\d{3,6})(?:\D|$)`)
	standardRe = regexp.MustCompile(`(?i)^\d*([a-zA-Z]{2,6})(\d{3,6})(?:\D|$)`)
)

func Parse(filename string) ParsedFile {
	ext := strings.ToLower(filepath.Ext(filename))
	name := strings.TrimSuffix(filename, filepath.Ext(filename))

	res := ParsedFile{Ext: ext}

	// 1. Extract site prefix: [site.com] or site.com@
	if m := bracketSiteRe.FindStringSubmatch(name); len(m) > 1 {
		res.SourceSite = strings.ToLower(m[1])
		name = name[len(m[0]):]
	} else if m := sitePrefixRe.FindStringSubmatch(name); len(m) > 1 {
		res.SourceSite = strings.ToLower(m[1])
		name = sitePrefixRe.ReplaceAllString(name, "")
	}

	// 2. Tokenize by non-alphanumeric characters
	tokens := tokenizeRe.Split(name, -1)
	var clean []string
	for _, t := range tokens {
		if t != "" {
			clean = append(clean, t)
		}
	}

	if len(clean) == 0 {
		return res
	}

	// 3. Build identifier from leading tokens
	idStart := -1
	for i, t := range clean {
		if hasLetter(t) {
			idStart = i
			break
		}
	}
	if idStart < 0 {
		return res
	}

	raw := strings.ToLower(clean[idStart])
	next := idStart + 1

	// If identifier ends with a letter, append the next pure-digit token (number part)
	if next < len(clean) && endsWithLetter(raw) && isPureDigits(clean[next]) && len(clean[next]) >= 3 {
		raw += clean[next]
		next++
	}

	// 4. Classify remaining tokens
	for i := next; i < len(clean); i++ {
		t := clean[i]

		if m := partTokenRe.FindStringSubmatch(t); len(m) > 1 {
			if res.Part == 0 {
				if p, err := strconv.Atoi(m[1]); err == nil {
					res.Part = p
				}
			}
			continue
		}

		if tagTokenRe.MatchString(t) {
			res.Tags = append(res.Tags, strings.ToLower(t))
			continue
		}

		if len(t) == 1 && hasLetter(t) && res.Part == 0 {
			res.Part = int(strings.ToUpper(t)[0]-'A') + 1
			continue
		}

		if isPureDigits(t) && len(t) <= 2 && res.Part == 0 {
			if p, err := strconv.Atoi(t); err == nil {
				res.Part = p
			}
			continue
		}
	}

	res.Tags = unique(res.Tags)

	// 5. Extract normalized number from raw identifier
	res.Number, res.RawNumber = extractNumber(raw)

	return res
}

// extractNumber returns (normalized number, raw matched portion).
func extractNumber(raw string) (string, string) {
	for _, entry := range []struct {
		re      *regexp.Regexp
		format  func([]string) string
	}{
		{heyzoRe, func(m []string) string { return strings.ToUpper(m[1]) + "-" + m[2] }},
		{mgstageRe, func(m []string) string { return strings.ToUpper(m[1]) + "-" + trimLeadingZeros(m[2]) }},
		{standardRe, func(m []string) string { return strings.ToUpper(m[1]) + "-" + trimLeadingZeros(m[2]) }},
	} {
		m := entry.re.FindStringSubmatch(raw)
		if len(m) <= 2 {
			continue
		}
		idx := entry.re.FindStringSubmatchIndex(raw)
		// idx[4] and idx[5] are the start/end of capture group 2 (the digit part)
		// rawMatch = everything from start up to end of the digit group
		rawMatch := strings.ToLower(raw[:idx[5]])
		return entry.format(m), rawMatch
	}
	return "", ""
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

func hasLetter(s string) bool {
	for _, c := range s {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') {
			return true
		}
	}
	return false
}

func endsWithLetter(s string) bool {
	if s == "" {
		return false
	}
	c := s[len(s)-1]
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func isPureDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
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
