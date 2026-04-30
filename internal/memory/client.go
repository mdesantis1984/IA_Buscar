package memory

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type Client struct {
	baseURL   string
	project   string
	apiKey    string
	client    *http.Client
}

type Observation struct {
	ID        int64     `json:"id,omitempty"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	Type      string    `json:"type"`
	TopicKey  string    `json:"topic_key,omitempty"`
	Project   string    `json:"project,omitempty"`
	CreatedAt time.Time `json:"createdAt,omitempty"`
}

func NewClient(baseURL, project, apiKey string) *Client {
	return &Client{
		baseURL: baseURL,
		project: project,
		apiKey:  apiKey,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *Client) Save(ctx context.Context, obs *Observation) error {
	if c.baseURL == "" {
		return nil
	}
	obs.Project = c.project
	data, err := json.Marshal(obs)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/mcp", bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("X-Api-Key", c.apiKey)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("memory save failed: %d", resp.StatusCode)
	}
	return nil
}

func (c *Client) Search(ctx context.Context, query string) ([]*Observation, error) {
	if c.baseURL == "" {
		return []*Observation{}, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/mcp?query="+query, nil)
	if err != nil {
		return nil, err
	}
	if c.apiKey != "" {
		req.Header.Set("X-Api-Key", c.apiKey)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("memory search failed: %d", resp.StatusCode)
	}
	var results []*Observation
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, err
	}
	return results, nil
}

func (c *Client) GetSession(ctx context.Context, sessionID string) (*Observation, error) {
	return nil, nil
}
