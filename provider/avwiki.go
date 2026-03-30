package provider

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

const (
	avwikiDirectURL = "https://av-wiki.net/%s/"
	avwikiSearchURL = "https://av-wiki.net/?s=%s&post_type=product"
)

type AVWiki struct {
	client *http.Client
}

func NewAVWiki() *AVWiki {
	return &AVWiki{client: &http.Client{Timeout: 15 * time.Second}}
}

func (a *AVWiki) Name() string { return "avwiki" }

func (a *AVWiki) Scrape(ctx context.Context, number string) (*MovieMetadata, error) {
	direct := fmt.Sprintf(avwikiDirectURL, strings.ToLower(number))
	if doc, err := a.fetchDoc(ctx, direct); err == nil {
		if meta := a.parseDetail(doc, number); meta != nil {
			return meta, nil
		}
	}

	search := fmt.Sprintf(avwikiSearchURL, url.QueryEscape(number))
	doc, err := a.fetchDoc(ctx, search)
	if err != nil {
		return nil, fmt.Errorf("avwiki search failed: %w", err)
	}
	detailURL := a.findFirstResult(doc)
	if detailURL == "" {
		return nil, fmt.Errorf("avwiki no result for %s", number)
	}
	detailDoc, err := a.fetchDoc(ctx, detailURL)
	if err != nil {
		return nil, fmt.Errorf("avwiki detail failed: %w", err)
	}
	meta := a.parseDetail(detailDoc, number)
	if meta == nil {
		return nil, fmt.Errorf("avwiki parse failed for %s", number)
	}
	return meta, nil
}

func (a *AVWiki) fetchDoc(ctx context.Context, target string) (*goquery.Document, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http %d", resp.StatusCode)
	}
	return goquery.NewDocumentFromReader(resp.Body)
}

func (a *AVWiki) findFirstResult(doc *goquery.Document) string {
	var result string
	doc.Find("article a, .read-more a").EachWithBreak(func(i int, s *goquery.Selection) bool {
		href, ok := s.Attr("href")
		if !ok || href == "" {
			return true
		}
		if strings.Contains(href, "av-wiki.net") {
			result = href
			return false
		}
		return true
	})
	return result
}

func (a *AVWiki) parseDetail(doc *goquery.Document, number string) *MovieMetadata {
	title := strings.TrimSpace(doc.Find("h1.entry-title").First().Text())
	if title == "" {
		title = strings.TrimSpace(doc.Find(".blockquote-like p").First().Text())
	}
	if title == "" {
		title, _ = doc.Find(".article-thumbnail a img").Attr("alt")
		title = strings.TrimSpace(title)
	}
	if title == "" {
		return nil
	}

	meta := &MovieMetadata{Number: number, Title: sanitizeTitle(title)}
	meta.CoverURL, _ = doc.Find(".article-thumbnail a img").First().Attr("src")

	premiered := strings.TrimSpace(doc.Find("dl.dltable dt:contains('配信開始日')").First().Next().Text())
	if premiered == "" {
		premiered = strings.TrimSpace(doc.Find("time").First().Text())
	}
	if len(premiered) >= 10 {
		meta.Premiered = premiered[:10]
		meta.Year = meta.Premiered[:4]
	}

	meta.Director = strings.TrimSpace(doc.Find("span[itemprop=director]").First().Text())
	meta.Maker = strings.TrimSpace(doc.Find("dl.dltable dt:contains('メーカー')").First().Next().Text())
	meta.Label = strings.TrimSpace(doc.Find("dl.dltable dt:contains('レーベル')").First().Next().Text())
	meta.Series = strings.TrimSpace(doc.Find("dl.dltable dt:contains('シリーズ')").First().Next().Text())

	doc.Find("dl.dltable dt:contains('AV女優名')").First().Next().Find("a").Each(func(i int, s *goquery.Selection) {
		name := strings.TrimSpace(s.Text())
		if name != "" {
			meta.Actors = append(meta.Actors, name)
		}
	})
	doc.Find("div.cat-link a").Each(func(i int, s *goquery.Selection) {
		name := strings.TrimSpace(s.Text())
		if name != "" {
			meta.Genres = append(meta.Genres, name)
		}
	})

	return meta
}
