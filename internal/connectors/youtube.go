package connectors

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/thiscloud/ia-buscar/internal/cache"
	"github.com/thiscloud/ia-buscar/internal/memory"
	"github.com/thiscloud/ia-buscar/pkg/types"
)

type YouTubeConnector struct {
	baseURL    string
	cacheSvc   *cache.Service
	memClient  *memory.Client
	httpClient *http.Client
}

func NewYouTubeConnector(cacheSvc *cache.Service, memClient *memory.Client) *YouTubeConnector {
	return &YouTubeConnector{
		baseURL:    "https://yewtu.be",
		cacheSvc:   cacheSvc,
		memClient:  memClient,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *YouTubeConnector) Name() string { return "youtube" }

func (c *YouTubeConnector) Search(ctx context.Context, req *types.SearchRequest) (*types.SearchResponse, error) {
	start := time.Now()
	query := strings.TrimSpace(req.Query)
	maxResults := getMaxResults(req.MaxResults, 10)

	cacheKey := cache.GenerateCacheKey(query, []string{"youtube"})
	if cached, ok, _ := c.cacheSvc.Get(ctx, cacheKey); ok {
		log.Printf("[youtube] cache hit for query: %s", query)
		cachedResp := &types.SearchResponse{}
		if err := json.Unmarshal(cached.Payload, cachedResp); err == nil {
			cachedResp.Cached = true
			return cachedResp, nil
		}
	}

	apiURL := fmt.Sprintf("%s/api/v1/search?q=%s&count=%d",
		c.baseURL, url.QueryEscape(query), maxResults)

	results, err := c.doYouTubeRequest(ctx, apiURL)
	if err != nil {
		log.Printf("[youtube] search error: %v", err)
	}

	resp := &types.SearchResponse{
		Query:       req.Query,
		Results:     results,
		SourcesUsed: []string{"youtube"},
		Cached:      false,
	}
	if len(results) == 0 && err != nil {
		resp.Warnings = []string{err.Error()}
	}

	c.saveToMemory(ctx, query, len(results), time.Since(start))
	c.cacheResults(ctx, cacheKey, resp)

	log.Printf("[youtube] search completed: query=%s, results=%d, latency=%v", query, len(results), time.Since(start))
	return resp, nil
}

func (c *YouTubeConnector) doYouTubeRequest(ctx context.Context, apiURL string) ([]types.SearchResultItem, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return []types.SearchResultItem{}, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return []types.SearchResultItem{}, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return []types.SearchResultItem{}, fmt.Errorf("API error: %d", resp.StatusCode)
	}

	var data []struct {
		VideoID      string `json:"videoId"`
		Title        string `json:"title"`
		Description  string `json:"description"`
		Author       string `json:"author"`
		Length       string `json:"length"`
		ViewCount    string `json:"viewCount"`
		Published    int64  `json:"published"`
		URL          string `json:"url"`
		Type         string `json:"type"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return []types.SearchResultItem{}, err
	}

	results := make([]types.SearchResultItem, 0, len(data))
	for _, item := range data {
		if item.Type != "video" {
			continue
		}

		snippet := item.Description
		if len(snippet) > 300 {
			snippet = snippet[:300] + "..."
		}

		videoURL := item.URL
		if videoURL == "" && item.VideoID != "" {
			videoURL = "https://www.youtube.com/watch?v=" + item.VideoID
		}

		publishedAt := time.Unix(item.Published, 0)

		results = append(results, types.SearchResultItem{
			Title:       item.Title,
			URL:         videoURL,
			Snippet:     snippet,
			Source:      "youtube",
			Type:        "video",
			Score:       parseViewCount(item.ViewCount),
			PublishedAt: &publishedAt,
			Author:      item.Author,
			Tags:        []string{},
			CitationID:  "youtube:" + item.VideoID,
		})
	}

	return results, nil
}

func parseViewCount(vc string) float64 {
	var count float64
	for _, c := range vc {
		if c >= '0' && c <= '9' {
			count = count*10 + float64(c-'0')
		}
	}
	return count
}

func (c *YouTubeConnector) saveToMemory(ctx context.Context, query string, count int, latency time.Duration) {
	if c.memClient == nil {
		return
	}
	c.memClient.Save(ctx, &memory.Observation{
		Title:    fmt.Sprintf("YouTube search: %s", query),
		Content:  fmt.Sprintf("**Query**: %s\n**Results**: %d\n**Latency**: %v", query, count, latency),
		Type:     "search",
		TopicKey: fmt.Sprintf("youtube-%s", sanitizeTopicKey(query)),
	})
}

func (c *YouTubeConnector) cacheResults(ctx context.Context, cacheKey string, resp *types.SearchResponse) {
	if c.cacheSvc == nil {
		return
	}
	data, err := json.Marshal(resp)
	if err != nil {
		return
	}
	c.cacheSvc.Set(ctx, cacheKey, data, resp.SourcesUsed)
}