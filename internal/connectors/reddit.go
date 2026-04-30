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

type RedditConnector struct {
	baseURL    string
	cacheSvc   *cache.Service
	memClient  *memory.Client
	httpClient *http.Client
}

func NewRedditConnector(cacheSvc *cache.Service, memClient *memory.Client) *RedditConnector {
	return &RedditConnector{
		baseURL:    "https://www.reddit.com",
		cacheSvc:   cacheSvc,
		memClient:  memClient,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *RedditConnector) Name() string { return "reddit" }

func (c *RedditConnector) Search(ctx context.Context, req *types.SearchRequest) (*types.SearchResponse, error) {
	start := time.Now()
	query := strings.TrimSpace(req.Query)
	maxResults := getMaxResults(req.MaxResults, 10)

	cacheKey := cache.GenerateCacheKey(query, []string{"reddit"})
	if cached, ok, _ := c.cacheSvc.Get(ctx, cacheKey); ok {
		log.Printf("[reddit] cache hit for query: %s", query)
		cachedResp := &types.SearchResponse{}
		if err := json.Unmarshal(cached.Payload, cachedResp); err == nil {
			cachedResp.Cached = true
			return cachedResp, nil
		}
	}

	apiURL := fmt.Sprintf("%s/search.json?q=%s&limit=%d",
		c.baseURL, url.QueryEscape(query), maxResults)

	results, err := c.doRedditRequest(ctx, apiURL)
	if err != nil {
		log.Printf("[reddit] search error: %v", err)
	}

	resp := &types.SearchResponse{
		Query:       req.Query,
		Results:     results,
		SourcesUsed: []string{"reddit"},
		Cached:      false,
	}
	if len(results) == 0 && err != nil {
		resp.Warnings = []string{err.Error()}
	}

	c.saveToMemory(ctx, query, len(results), time.Since(start))
	c.cacheResults(ctx, cacheKey, resp)

	log.Printf("[reddit] search completed: query=%s, results=%d, latency=%v", query, len(results), time.Since(start))
	return resp, nil
}

func (c *RedditConnector) doRedditRequest(ctx context.Context, apiURL string) ([]types.SearchResultItem, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return []types.SearchResultItem{}, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "ia-buscar/1.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return []types.SearchResultItem{}, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return []types.SearchResultItem{}, fmt.Errorf("API error: %d", resp.StatusCode)
	}

	var data struct {
		Data struct {
			Children []struct {
				Data struct {
					Title      string `json:"title"`
					URL        string `json:"url"`
					Subreddit  string `json:"subreddit"`
					Score      int    `json:"score"`
					NumComments int    `json:"num_comments"`
					Author     string `json:"author"`
					CreatedUTC float64 `json:"created_utc"`
					SelfText   string `json:"selftext"`
					Permalink  string `json:"permalink"`
				} `json:"data"`
			} `json:"children"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return []types.SearchResultItem{}, err
	}

	results := make([]types.SearchResultItem, 0, len(data.Data.Children))
	for _, child := range data.Data.Children {
		item := child.Data

		snippet := item.SelfText
		if snippet == "" {
			snippet = fmt.Sprintf("Score: %d | Comments: %d", item.Score, item.NumComments)
		} else if len(snippet) > 300 {
			snippet = snippet[:300] + "..."
		}

		postURL := item.URL
		if !strings.HasPrefix(postURL, "http") {
			postURL = "https://reddit.com" + item.Permalink
		}

		publishedAt := time.Unix(int64(item.CreatedUTC), 0)

		results = append(results, types.SearchResultItem{
			Title:       item.Title,
			URL:         postURL,
			Snippet:     snippet,
			Source:      "reddit",
			Type:        "post",
			Score:       float64(item.Score),
			PublishedAt: &publishedAt,
			Author:      item.Author,
			Tags:        []string{item.Subreddit},
			CitationID:  "reddit:" + item.Permalink,
		})
	}

	return results, nil
}

func (c *RedditConnector) saveToMemory(ctx context.Context, query string, count int, latency time.Duration) {
	if c.memClient == nil {
		return
	}
	c.memClient.Save(ctx, &memory.Observation{
		Title:    fmt.Sprintf("Reddit search: %s", query),
		Content:  fmt.Sprintf("**Query**: %s\n**Results**: %d\n**Latency**: %v", query, count, latency),
		Type:     "search",
		TopicKey: fmt.Sprintf("reddit-%s", sanitizeTopicKey(query)),
	})
}

func (c *RedditConnector) cacheResults(ctx context.Context, cacheKey string, resp *types.SearchResponse) {
	if c.cacheSvc == nil {
		return
	}
	data, err := json.Marshal(resp)
	if err != nil {
		return
	}
	c.cacheSvc.Set(ctx, cacheKey, data, resp.SourcesUsed)
}