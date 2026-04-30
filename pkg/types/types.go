package types

import (
	"context"
	"time"
)

type SearchRequest struct {
	Query       string            `json:"query"`
	Sources     []string          `json:"sources,omitempty"`
	MaxResults  int               `json:"maxResults,omitempty"`
	Language    string            `json:"language,omitempty"`
	SafeSearch  bool              `json:"safeSearch,omitempty"`
	TimeRange   string            `json:"timeRange,omitempty"`
	Format      string            `json:"format,omitempty"`
	CachePolicy string            `json:"cachePolicy,omitempty"`
	DeepResearch bool             `json:"deepResearch,omitempty"`
	Filters     map[string]string `json:"filters,omitempty"`
}

type SearchResultItem struct {
	Title       string    `json:"title"`
	URL         string    `json:"url"`
	Snippet     string    `json:"snippet,omitempty"`
	Source      string    `json:"source"`
	Type        string    `json:"type,omitempty"`
	Score       float64   `json:"score,omitempty"`
	PublishedAt *time.Time `json:"publishedAt,omitempty"`
	Author      string    `json:"author,omitempty"`
	Tags        []string  `json:"tags,omitempty"`
	CitationID  string    `json:"citationId,omitempty"`
	CanonicalURL string   `json:"canonicalUrl,omitempty"`
}

type SearchResponse struct {
	Query       string              `json:"query"`
	Results     []SearchResultItem `json:"results,omitempty"`
	Summary     string              `json:"summary,omitempty"`
	KeyFindings []string            `json:"keyFindings,omitempty"`
	SourcesUsed []string           `json:"sourcesUsed,omitempty"`
	Confidence  float64           `json:"confidence,omitempty"`
	Cached      bool               `json:"cached,omitempty"`
	Partial     bool               `json:"partial,omitempty"`
	Warnings    []string           `json:"warnings,omitempty"`
	Errors      []string           `json:"errors,omitempty"`
}

type FetchResponse struct {
	URL       string            `json:"url"`
	Title     string            `json:"title,omitempty"`
	Content   string            `json:"content,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	Warnings  []string          `json:"warnings,omitempty"`
}

type SearchConnector interface {
	Name() string
	Search(ctx context.Context, req *SearchRequest) (*SearchResponse, error)
}

type CacheEntry struct {
	CacheKey   string    `json:"cacheKey"`
	CreatedAt  time.Time `json:"createdAt"`
	ExpiresAt  time.Time `json:"expiresAt"`
	Payload    []byte    `json:"payload,omitempty"`
	SourceSet  []string  `json:"sourceSet,omitempty"`
}
