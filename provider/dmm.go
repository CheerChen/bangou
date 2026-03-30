package provider

import (
	"context"
	"encoding/json"
	"fmt"
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

func (d *DMM) Scrape(ctx context.Context, number string) (*MovieMetadata, error) {
	if d.apiID == "" || d.affiliateID == "" {
		return nil, fmt.Errorf("dmm not configured")
	}
	label, num, err := splitNumber(number)
	if err != nil {
		return nil, err
	}

	keywords := []string{
		fmt.Sprintf("%s00%s", strings.ToLower(label), num),
		fmt.Sprintf("%s%s", strings.ToLower(label), num),
		number,
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

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}

	resp, err := d.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http %d", resp.StatusCode)
	}

	var result dmmResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	if result.Result.Status != 200 || len(result.Result.Items) == 0 {
		return nil, nil
	}
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
	m.Runtime = item.Runtime

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
	Title    string      `json:"title"`
	Date     string      `json:"date"`
	Runtime  string      `json:"runtime"`
	ImageURL dmmImageURL `json:"imageURL"`
	ItemInfo dmmItemInfo `json:"iteminfo"`
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
