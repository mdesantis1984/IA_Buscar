package connectors

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/thiscloud/ia-buscar/internal/cache"
	"github.com/thiscloud/ia-buscar/internal/memory"
	"github.com/thiscloud/ia-buscar/pkg/types"
)

type ImagesConnector struct {
	baseURL    string
	cacheSvc   *cache.Service
	memClient  *memory.Client
	httpClient *http.Client
}

func NewImagesConnector(cacheSvc *cache.Service, memClient *memory.Client) *ImagesConnector {
	return &ImagesConnector{
		baseURL:    "https://duckduckgo.com",
		cacheSvc:   cacheSvc,
		memClient:  memClient,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *ImagesConnector) Name() string { return "images" }

func (c *ImagesConnector) Search(ctx context.Context, req *types.SearchRequest) (*types.SearchResponse, error) {
	start := time.Now()
	query := strings.TrimSpace(req.Query)
	maxResults := getMaxResults(req.MaxResults, 10)

	cacheKey := cache.GenerateCacheKey(query, []string{"images"})
	if cached, ok, _ := c.cacheSvc.Get(ctx, cacheKey); ok {
		log.Printf("[images] cache hit for query: %s", query)
		cachedResp := &types.SearchResponse{}
		if err := json.Unmarshal(cached.Payload, cachedResp); err == nil {
			cachedResp.Cached = true
			return cachedResp, nil
		}
	}

	results, err := c.doImagesRequest(ctx, query, maxResults)
	if err != nil {
		log.Printf("[images] search error: %v", err)
	}

	resp := &types.SearchResponse{
		Query:       req.Query,
		Results:     results,
		SourcesUsed: []string{"images"},
		Cached:      false,
	}
	if len(results) == 0 && err != nil {
		resp.Warnings = []string{err.Error()}
	}

	c.saveToMemory(ctx, query, len(results), time.Since(start))
	c.cacheResults(ctx, cacheKey, resp)

	log.Printf("[images] search completed: query=%s, results=%d, latency=%v", query, len(results), time.Since(start))
	return resp, nil
}

func (c *ImagesConnector) doImagesRequest(ctx context.Context, query string, maxResults int) ([]types.SearchResultItem, error) {
	apiURL := fmt.Sprintf("%s/?q=%s&ia=images&iax=1",
		c.baseURL, url.QueryEscape(query))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return []types.SearchResultItem{}, err
	}
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return []types.SearchResultItem{}, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return []types.SearchResultItem{}, fmt.Errorf("HTTP error: %d", resp.StatusCode)
	}

	body, err := readResponseBody(resp)
	if err != nil {
		return []types.SearchResultItem{}, err
	}

	results, err := parseImageResults(body, maxResults)
	if err != nil {
		return []types.SearchResultItem{}, err
	}
	return results, nil
}

func readResponseBody(resp *http.Response) (string, error) {
	buf := make([]byte, 0, 65536)
	tmp := make([]byte, 4096)
	reader := resp.Body
	for {
		n, err := reader.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
			if len(buf) > 1024*1024 {
				break
			}
		}
		if err != nil {
			break
		}
	}
	return string(buf), nil
}

var (
	imageTitleRegex = regexp.MustCompile(`data-title="([^"]*)"`)
	imageURLRegex   = regexp.MustCompile(`data-image="([^"]*)"`)
	imageThumbRegex = regexp.MustCompile(`data-thumb="([^"]*)"`)
	imageSourceRegex = regexp.MustCompile(`data-source="([^"]*)"`)
)

func parseImageResults(html string, maxResults int) ([]types.SearchResultItem, error) {
	titles := imageTitleRegex.FindAllStringSubmatch(html, -1)
	urls := imageURLRegex.FindAllStringSubmatch(html, -1)
	thumbs := imageThumbRegex.FindAllStringSubmatch(html, -1)
	sources := imageSourceRegex.FindAllStringSubmatch(html, -1)

	minLen := min(len(titles), min(len(urls), min(len(thumbs), len(sources))))
	if minLen == 0 {
		return []types.SearchResultItem{}, nil
	}

	if minLen > maxResults {
		minLen = maxResults
	}

	results := make([]types.SearchResultItem, 0, minLen)
	for i := 0; i < minLen; i++ {
		title := strings.TrimSpace(titles[i][1])
		imageURL := strings.TrimSpace(urls[i][1])
		thumbURL := strings.TrimSpace(thumbs[i][1])
		source := strings.TrimSpace(sources[i][1])

		if imageURL == "" || title == "" {
			continue
		}

		cdnURL := thumbURL
		if cdnURL == "" {
			cdnURL = imageURL
		}

		results = append(results, types.SearchResultItem{
			Title:       title,
			URL:         imageURL,
			Snippet:     fmt.Sprintf("Source: %s", source),
			Source:      "duckduckgo-images",
			Type:        "image",
			Score:       1.0,
			PublishedAt: nil,
			Author:      source,
			Tags:        []string{},
			CitationID:  fmt.Sprintf("ddg:%d", i),
			CanonicalURL: cdnURL,
		})
	}

	return results, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (c *ImagesConnector) saveToMemory(ctx context.Context, query string, count int, latency time.Duration) {
	if c.memClient == nil {
		return
	}
	c.memClient.Save(ctx, &memory.Observation{
		Title:    fmt.Sprintf("Images search: %s", query),
		Content:  fmt.Sprintf("**Query**: %s\n**Results**: %d\n**Latency**: %v", query, count, latency),
		Type:     "search",
		TopicKey: fmt.Sprintf("images-%s", sanitizeTopicKey(query)),
	})
}

func (c *ImagesConnector) cacheResults(ctx context.Context, cacheKey string, resp *types.SearchResponse) {
	if c.cacheSvc == nil {
		return
	}
	data, err := json.Marshal(resp)
	if err != nil {
		return
	}
	c.cacheSvc.Set(ctx, cacheKey, data, resp.SourcesUsed)
}