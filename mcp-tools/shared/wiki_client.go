package shared

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

type SearchResult struct {
	Path    string  `json:"path"`
	Title   string  `json:"title"`
	Snippet string  `json:"snippet"`
	Score   float64 `json:"score"`
}

type PageEntry struct {
	Name  string `json:"name"`
	Title string `json:"title"`
	Path  string `json:"path"`
}

type WikiClient struct {
	baseURL    string
	httpClient *http.Client
}

func NewWikiClient(baseURL string) *WikiClient {
	return &WikiClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *WikiClient) GetPage(pagePath string) (string, error) {
	resp, err := c.httpClient.Get(fmt.Sprintf("%s/wiki/%s", c.baseURL, pagePath))
	if err != nil {
		return "", fmt.Errorf("wiki request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return "", fmt.Errorf("page not found: %s", pagePath)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("wiki returned status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}
	return string(body), nil
}

func (c *WikiClient) Search(query string) ([]SearchResult, error) {
	resp, err := c.httpClient.Get(fmt.Sprintf("%s/search?q=%s", c.baseURL, url.QueryEscape(query)))
	if err != nil {
		return nil, fmt.Errorf("search request failed: %w", err)
	}
	defer resp.Body.Close()
	var results []SearchResult
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, fmt.Errorf("failed to decode search results: %w", err)
	}
	return results, nil
}

func (c *WikiClient) ListPages(category string) ([]PageEntry, error) {
	resp, err := c.httpClient.Get(fmt.Sprintf("%s/wiki/%s", c.baseURL, category))
	if err != nil {
		return nil, fmt.Errorf("list request failed: %w", err)
	}
	defer resp.Body.Close()
	var entries []PageEntry
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		return nil, fmt.Errorf("failed to decode page list: %w", err)
	}
	return entries, nil
}
