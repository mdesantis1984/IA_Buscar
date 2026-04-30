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

type AcademicConnector struct {
	baseURL    string
	cacheSvc   *cache.Service
	memClient  *memory.Client
	httpClient *http.Client
}

func NewAcademicConnector(cacheSvc *cache.Service, memClient *memory.Client) *AcademicConnector {
	return &AcademicConnector{
		baseURL:    "https://api.semanticscholar.org/graph/v1",
		cacheSvc:   cacheSvc,
		memClient:  memClient,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *AcademicConnector) Name() string { return "academic" }

func (c *AcademicConnector) Search(ctx context.Context, req *types.SearchRequest) (*types.SearchResponse, error) {
	start := time.Now()
	query := strings.TrimSpace(req.Query)
	maxResults := getMaxResults(req.MaxResults, 10)

	cacheKey := cache.GenerateCacheKey(query, []string{"academic"})
	if cached, ok, _ := c.cacheSvc.Get(ctx, cacheKey); ok {
		log.Printf("[academic] cache hit for query: %s", query)
		cachedResp := &types.SearchResponse{}
		if err := json.Unmarshal(cached.Payload, cachedResp); err == nil {
			cachedResp.Cached = true
			return cachedResp, nil
		}
	}

	apiURL := fmt.Sprintf("%s/paper/search?query=%s&limit=%d&fields=title,abstract,year,citationCount,authors,externalIds",
		c.baseURL, url.QueryEscape(query), maxResults)

	results, err := c.doAcademicRequest(ctx, apiURL)
	if err != nil {
		log.Printf("[academic] search error: %v", err)
	}

	resp := &types.SearchResponse{
		Query:       req.Query,
		Results:     results,
		SourcesUsed: []string{"academic"},
		Cached:      false,
	}
	if len(results) == 0 && err != nil {
		resp.Warnings = []string{err.Error()}
	}

	c.saveToMemory(ctx, query, len(results), time.Since(start))
	c.cacheResults(ctx, cacheKey, resp)

	log.Printf("[academic] search completed: query=%s, results=%d, latency=%v", query, len(results), time.Since(start))
	return resp, nil
}

func (c *AcademicConnector) doAcademicRequest(ctx context.Context, apiURL string) ([]types.SearchResultItem, error) {
	time.Sleep(1 * time.Second)

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

	if resp.StatusCode == http.StatusTooManyRequests {
		return []types.SearchResultItem{}, fmt.Errorf("rate limited")
	}
	if resp.StatusCode >= 400 {
		return []types.SearchResultItem{}, fmt.Errorf("API error: %d", resp.StatusCode)
	}

	var data struct {
		Data []struct {
			PaperID       string   `json:"paperId"`
			Title        string   `json:"title"`
			Abstract     string   `json:"abstract"`
			Year         int      `json:"year"`
			CitationCount int     `json:"citationCount"`
			Authors      []struct {
				Name string `json:"name"`
			} `json:"authors"`
			ExternalIDs struct {
				DOI string `json:"doi"`
			} `json:"externalIds"`
		} `json:"data"`
		Total int `json:"total"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return []types.SearchResultItem{}, err
	}

	results := make([]types.SearchResultItem, 0, len(data.Data))
	for _, item := range data.Data {
		snippet := item.Abstract
		if len(snippet) > 300 {
			snippet = snippet[:300] + "..."
		}

		var author string
		if len(item.Authors) > 0 {
			author = item.Authors[0].Name
			if len(item.Authors) > 1 {
				author += fmt.Sprintf(" et al.")
			}
		}

		var publishedAt *time.Time
		if item.Year > 0 {
			t := time.Date(item.Year, 1, 1, 0, 0, 0, 0, time.UTC)
			publishedAt = &t
		}

		url := "https://www.semanticscholar.org/paper/" + item.PaperID
		if item.ExternalIDs.DOI != "" {
			url = "https://doi.org/" + item.ExternalIDs.DOI
		}

		results = append(results, types.SearchResultItem{
			Title:       item.Title,
			URL:         url,
			Snippet:     snippet,
			Source:      "semantic-scholar",
			Type:        "paper",
			Score:       float64(item.CitationCount),
			PublishedAt: publishedAt,
			Author:      author,
			Tags:        []string{},
			CitationID:  "ss:" + item.PaperID,
		})
	}

	return results, nil
}

func (c *AcademicConnector) saveToMemory(ctx context.Context, query string, count int, latency time.Duration) {
	if c.memClient == nil {
		return
	}
	c.memClient.Save(ctx, &memory.Observation{
		Title:    fmt.Sprintf("Academic search: %s", query),
		Content:  fmt.Sprintf("**Query**: %s\n**Results**: %d\n**Latency**: %v", query, count, latency),
		Type:     "search",
		TopicKey: fmt.Sprintf("academic-%s", sanitizeTopicKey(query)),
	})
}

func (c *AcademicConnector) cacheResults(ctx context.Context, cacheKey string, resp *types.SearchResponse) {
	if c.cacheSvc == nil {
		return
	}
	data, err := json.Marshal(resp)
	if err != nil {
		return
	}
	c.cacheSvc.Set(ctx, cacheKey, data, resp.SourcesUsed)
}