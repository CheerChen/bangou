package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const dmmAPIURL = "https://api.dmm.com/affiliate/v3/ItemList"

var dmmIdentifierRe = regexp.MustCompile(`(?i)([a-z]{2,6})(\d{3,6})`)

type DMM struct {
	apiID       string
	affiliateID string
	client      *http.Client
}

func NewDMM(apiID, affiliateID string) *DMM {
	return &DMM{
		apiID:       strings.TrimSpace(apiID),
		affiliateID: strings.TrimSpace(affiliateID),
		client:      &http.Client{Timeout: 15 * time.Second},
	}
}

func (d *DMM) Name() string { return "dmm" }

func (d *DMM) Scrape(ctx context.Context, p Predict) (*MovieMetadata, error) {
	if d.apiID == "" || d.affiliateID == "" {
		return nil, fmt.Errorf("dmm not configured")
	}
	number := p.Number
	label, num, err := splitNumber(number)
	if err != nil {
		return nil, err
	}

	keywords := []string{
		fmt.Sprintf("%s00%s", strings.ToLower(label), num),
		fmt.Sprintf("%s%s", strings.ToLower(label), num),
		number,
	}
	// Prepend RawNumber if it provides a distinct keyword (e.g. "13dsvr01801")
	if p.RawNumber != "" {
		seen := make(map[string]bool, len(keywords))
		for _, kw := range keywords {
			seen[strings.ToLower(kw)] = true
		}
		if !seen[p.RawNumber] {
			keywords = append([]string{p.RawNumber}, keywords...)
		}
	}

	for _, kw := range keywords {
		item, err := d.searchOnce(ctx, kw)
		if err != nil {
			continue
		}
		if item != nil {
			return d.toMetadata(number, item), nil
		}
	}
	return nil, fmt.Errorf("dmm no results for %s", number)
}

func (d *DMM) searchOnce(ctx context.Context, keyword string) (*dmmItem, error) {
	u, _ := url.Parse(dmmAPIURL)
	q := u.Query()
	q.Set("api_id", d.apiID)
	q.Set("affiliate_id", d.affiliateID)
	q.Set("site", "FANZA")
	q.Set("keyword", keyword)
	q.Set("output", "json")
	u.RawQuery = q.Encode()

	log.Printf("[dmm] GET keyword=%s", keyword)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}

	resp, err := d.client.Do(req)
	if err != nil {
		log.Printf("[dmm] GET keyword=%s -> error: %v", keyword, err)
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		log.Printf("[dmm] GET keyword=%s -> HTTP %d", keyword, resp.StatusCode)
		return nil, fmt.Errorf("http %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var result dmmResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	if result.Result.Status != 200 || len(result.Result.Items) == 0 {
		log.Printf("[dmm] GET keyword=%s -> 0 items", keyword)
		return nil, nil
	}
	log.Printf("[dmm] GET keyword=%s -> %d items", keyword, len(result.Result.Items))

	item := selectDMMItem(keyword, result.Result.Items)
	if item == nil {
		log.Printf("[dmm] GET keyword=%s -> 0 matched items after filtering", keyword)
		return nil, nil
	}
	item.rawBody = body
	return item, nil
}

func (d *DMM) toMetadata(number string, item *dmmItem) *MovieMetadata {
	m := &MovieMetadata{Number: number, Title: sanitizeTitle(item.Title)}

	if item.Date != "" {
		m.Premiered = strings.Split(item.Date, " ")[0]
		if len(m.Premiered) >= 4 {
			m.Year = m.Premiered[:4]
		}
	}
	m.Runtime = item.Volume // "86" = 86 minutes
	m.ContentID = item.ContentID
	m.PageURL = item.URL
	m.Rating = item.Review.Average
	m.ReviewCount = item.Review.Count

	for _, a := range item.ItemInfo.Actress {
		if a.Name != "" {
			m.Actors = append(m.Actors, a.Name)
		}
	}
	for _, g := range item.ItemInfo.Genre {
		if g.Name != "" {
			m.Genres = append(m.Genres, g.Name)
		}
	}
	if len(item.ItemInfo.Director) > 0 {
		m.Director = item.ItemInfo.Director[0].Name
	}
	if len(item.ItemInfo.Maker) > 0 {
		m.Maker = item.ItemInfo.Maker[0].Name
	}
	if len(item.ItemInfo.Label) > 0 {
		m.Label = item.ItemInfo.Label[0].Name
	}
	if len(item.ItemInfo.Series) > 0 {
		m.Series = item.ItemInfo.Series[0].Name
	}
	if item.ImageURL.Large != "" {
		m.CoverURL = item.ImageURL.Large
	} else if item.ImageURL.Small != "" {
		m.CoverURL = dmmGuessLargeCover(item.ImageURL.Small)
	}
	if len(item.SampleImageURL.SampleL.Image) > 0 {
		m.SampleImages = item.SampleImageURL.SampleL.Image
	} else if len(item.SampleImageURL.SampleS.Image) > 0 {
		m.SampleImages = dmmGuessLargeSamples(item.SampleImageURL.SampleS.Image)
	}
	m.SampleMovieURL = item.SampleMovieURL.Largest()
	m.RawJSON = item.rawBody
	return m
}

type dmmResponse struct {
	Result dmmResult `json:"result"`
}

type dmmResult struct {
	Status int       `json:"status"`
	Items  []dmmItem `json:"items"`
}

type dmmItem struct {
	ContentID      string            `json:"content_id"`
	ProductID      string            `json:"product_id"`
	ServiceCode    string            `json:"service_code"`
	FloorCode      string            `json:"floor_code"`
	Title          string            `json:"title"`
	Date           string            `json:"date"`
	Volume         string            `json:"volume"` // runtime in minutes
	URL            string            `json:"URL"`
	Review         dmmReview         `json:"review"`
	ImageURL       dmmImageURL       `json:"imageURL"`
	SampleImageURL dmmSampleImageURL `json:"sampleImageURL"`
	SampleMovieURL dmmSampleMovieURL `json:"sampleMovieURL"`
	ItemInfo       dmmItemInfo       `json:"iteminfo"`
	rawBody        []byte
}

type dmmReview struct {
	Count   int    `json:"count"`
	Average string `json:"average"`
}

type dmmSampleImageURL struct {
	SampleS struct {
		Image []string `json:"image"`
	} `json:"sample_s"`
	SampleL struct {
		Image []string `json:"image"`
	} `json:"sample_l"`
}

type dmmSampleMovieURL struct {
	Size476  string `json:"size_476_306"`
	Size560  string `json:"size_560_360"`
	Size644  string `json:"size_644_414"`
	Size720  string `json:"size_720_480"`
}

func (s dmmSampleMovieURL) Largest() string {
	for _, u := range []string{s.Size720, s.Size644, s.Size560, s.Size476} {
		if u != "" {
			return u
		}
	}
	return ""
}

type dmmImageURL struct {
	Small string `json:"small"`
	Large string `json:"large"`
}

type dmmItemInfo struct {
	Genre    []dmmNameID `json:"genre"`
	Maker    []dmmNameID `json:"maker"`
	Actress  []dmmNameID `json:"actress"`
	Director []dmmNameID `json:"director"`
	Label    []dmmNameID `json:"label"`
	Series   []dmmNameID `json:"series"`
}

type dmmNameID struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// dmmFloorPriority returns a lower number for more preferred service/floor combos.
// digital/videoa (streaming video) has the richest metadata.
func dmmFloorPriority(item *dmmItem) int {
	switch item.ServiceCode {
	case "digital":
		return 0 // best: digital video
	case "monthly":
		return 1 // good: monthly subscription
	case "mono":
		return 2 // fallback: physical/DVD
	default:
		return 3
	}
}

func selectDMMItem(keyword string, items []dmmItem) *dmmItem {
	matchKeys := buildDMMMatchKeys(keyword)
	var best *dmmItem
	bestPri := 99
	for i := range items {
		if !dmmItemMatches(matchKeys, &items[i]) {
			continue
		}
		pri := dmmFloorPriority(&items[i])
		if best == nil || pri < bestPri {
			best = &items[i]
			bestPri = pri
		}
	}
	return best
}

func dmmItemMatches(matchKeys map[string]struct{}, item *dmmItem) bool {
	if len(matchKeys) == 0 || item == nil {
		return false
	}
	for _, field := range []string{item.ContentID, item.ProductID} {
		for key := range buildDMMMatchKeys(field) {
			if _, ok := matchKeys[key]; ok {
				return true
			}
		}
	}
	return false
}

func buildDMMMatchKeys(s string) map[string]struct{} {
	compact := compactDMMIdentifier(s)
	if compact == "" {
		return nil
	}

	keys := map[string]struct{}{
		compact: {},
	}

	for _, match := range dmmIdentifierRe.FindAllStringSubmatch(compact, -1) {
		if len(match) < 3 {
			continue
		}
		keys[strings.ToLower(match[1])+trimDMMLeadingZeros(match[2])] = struct{}{}
	}
	return keys
}

// dmmGuessLargeCover derives a large cover URL from a small one: ps.jpg → pl.jpg
func dmmGuessLargeCover(small string) string {
	if i := strings.LastIndex(small, "ps."); i >= 0 {
		return small[:i] + "pl." + small[i+3:]
	}
	return small
}

// dmmGuessLargeSamples derives large sample URLs from small ones by inserting "jp" before the last "-".
// e.g. dass00185-1.jpg → dass00185jp-1.jpg
func dmmGuessLargeSamples(smalls []string) []string {
	out := make([]string, len(smalls))
	for i, s := range smalls {
		if j := strings.LastIndex(s, "-"); j >= 0 {
			out[i] = s[:j] + "jp" + s[j:]
		} else {
			out[i] = s
		}
	}
	return out
}

func compactDMMIdentifier(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r + ('a' - 'A'))
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		}
	}
	return b.String()
}

func trimDMMLeadingZeros(s string) string {
	n, err := strconv.Atoi(s)
	if err != nil {
		return s
	}
	return strconv.Itoa(n)
}
