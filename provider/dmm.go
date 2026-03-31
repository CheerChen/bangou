package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const dmmAPIURL = "https://api.dmm.com/affiliate/v3/ItemList"

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
	result.Result.Items[0].rawBody = body
	return &result.Result.Items[0], nil
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
	} else {
		m.CoverURL = item.ImageURL.Small
	}
	// Prefer large sample images, fallback to small
	if len(item.SampleImageURL.SampleL.Image) > 0 {
		m.SampleImages = item.SampleImageURL.SampleL.Image
	} else if len(item.SampleImageURL.SampleS.Image) > 0 {
		m.SampleImages = item.SampleImageURL.SampleS.Image
	}
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
	Title          string            `json:"title"`
	Date           string            `json:"date"`
	Volume         string            `json:"volume"` // runtime in minutes
	URL            string            `json:"URL"`
	Review         dmmReview         `json:"review"`
	ImageURL       dmmImageURL       `json:"imageURL"`
	SampleImageURL dmmSampleImageURL `json:"sampleImageURL"`
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
