package provider

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

type MetaTube struct {
	apiURL string
	token  string
	client *http.Client
}

func NewMetaTube(apiURL, token string) *MetaTube {
	return &MetaTube{apiURL: apiURL, token: token, client: &http.Client{Timeout: 15 * time.Second}}
}

func (m *MetaTube) Name() string { return "metatube" }

func (m *MetaTube) Scrape(ctx context.Context, number string) (*MovieMetadata, error) {
	_ = ctx
	_ = number
	return nil, fmt.Errorf("metatube not implemented")
}
