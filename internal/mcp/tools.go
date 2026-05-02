package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/thiscloud/ia-buscar/internal/connectors"
	"github.com/thiscloud/ia-buscar/internal/search"
	"github.com/thiscloud/ia-buscar/pkg/types"
)

func (s *Server) registerTools() {
	for i := range s.toolsRegistry {
		t := &s.toolsRegistry[i]
		switch t.Name {
		case "search_web":
			t.Handler = s.makeSearchHandler("web")
		case "search_github":
			t.Handler = s.makeSearchHandler("github")
		case "search_github_pr":
			t.Handler = s.makeGitHubPRHandler()
		case "search_github_issue":
			t.Handler = s.makeGitHubIssueHandler()
		case "search_stackoverflow":
			t.Handler = s.makeSearchHandler("stackoverflow")
		case "search_npm":
			t.Handler = s.makeSearchHandler("npm")
		case "search_nuget":
			t.Handler = s.makeSearchHandler("nuget")
		case "search_pypi":
			t.Handler = s.makeSearchHandler("pypi")
		case "search_docker_hub":
			t.Handler = s.makeSearchHandler("dockerhub")
		case "search_doc_oficial", "search_local_index":
			t.Handler = s.makeSearchHandler("web")
		case "search_academic":
			t.Handler = s.makeSearchHandler("academic")
		case "search_reddit":
			t.Handler = s.makeSearchHandler("reddit")
		case "search_youtube":
			t.Handler = s.makeSearchHandler("youtube")
		case "search_images":
			t.Handler = s.makeSearchHandler("images")
		case "search_news":
			t.Handler = s.makeSearchHandler("news")
		case "fetch_url":
			t.Handler = s.makeFetchHandler("fetch")
		case "fetch_and_extract":
			t.Handler = s.makeFetchHandler("fetch_and_extract")
		case "extract_structured":
			t.Handler = s.makeFetchHandler("extract_structured")
		case "validate_url":
			t.Handler = s.makeValidateHandler("validate_url")
		case "check_link_status":
			t.Handler = s.makeValidateHandler("check_link_status")
		case "summarize_results":
			t.Handler = s.makeSynthesizeHandler("summarize")
		case "deep_research":
			t.Handler = s.makeSynthesizeHandler("deep_research")
		case "compare_sources":
			t.Handler = s.makeSynthesizeHandler("compare_sources")
		case "get_cached":
			t.Handler = s.makeCacheHandler("get")
		case "invalidate_cache":
			t.Handler = s.makeCacheHandler("invalidate")
		case "get_search_history":
			t.Handler = s.makeCacheHandler("history")
		case "get_current_date":
			t.Handler = s.getCurrentDateHandler
		}
	}
}

func (s *Server) makeSearchHandler(source string) func(ctx context.Context, args json.RawMessage) (interface{}, error) {
	return func(ctx context.Context, args json.RawMessage) (interface{}, error) {
		var req types.SearchRequest
		if err := json.Unmarshal(args, &req); err != nil {
			return nil, fmt.Errorf("invalid args: %w", err)
		}
		if s.connectorManager == nil {
			return &types.SearchResponse{
				Query:       req.Query,
				Results:     []types.SearchResultItem{},
				Errors:     []string{"connector manager not initialized"},
			}, nil
		}

		var plan *search.SearchPlan
		if s.planner != nil {
			plan = s.planner.Plan(req.Query, &req)
		}

		resp, err := s.connectorManager.Search(ctx, source, &req)
		if err != nil {
			return nil, err
		}

		if plan != nil {
			if len(plan.Connectors) > 0 {
				resp.SourcesUsed = []string{source}
			}
			if plan.Intent != "" {
				resp.Warnings = append(resp.Warnings, "intent: "+plan.Intent)
			}
		}

		return resp, nil
	}
}

func (s *Server) makeGitHubPRHandler() func(ctx context.Context, args json.RawMessage) (interface{}, error) {
	return func(ctx context.Context, args json.RawMessage) (interface{}, error) {
		var req types.SearchRequest
		if err := json.Unmarshal(args, &req); err != nil {
			return nil, fmt.Errorf("invalid args: %w", err)
		}
		conn, ok := s.connectorManager.GetConnector("github")
		if !ok {
			return &types.SearchResponse{
				Query:   req.Query,
				Results: []types.SearchResultItem{},
				Errors:  []string{"github connector not available"},
			}, nil
		}
		ghConn, ok := conn.(*connectors.GitHubConnector)
		if !ok {
			return &types.SearchResponse{
				Query:   req.Query,
				Results: []types.SearchResultItem{},
				Errors:  []string{"invalid github connector type"},
			}, nil
		}
		return ghConn.SearchPR(ctx, &req)
	}
}

func (s *Server) makeGitHubIssueHandler() func(ctx context.Context, args json.RawMessage) (interface{}, error) {
	return func(ctx context.Context, args json.RawMessage) (interface{}, error) {
		var req types.SearchRequest
		if err := json.Unmarshal(args, &req); err != nil {
			return nil, fmt.Errorf("invalid args: %w", err)
		}
		conn, ok := s.connectorManager.GetConnector("github")
		if !ok {
			return &types.SearchResponse{
				Query:   req.Query,
				Results: []types.SearchResultItem{},
				Errors:  []string{"github connector not available"},
			}, nil
		}
		ghConn, ok := conn.(*connectors.GitHubConnector)
		if !ok {
			return &types.SearchResponse{
				Query:   req.Query,
				Results: []types.SearchResultItem{},
				Errors:  []string{"invalid github connector type"},
			}, nil
		}
		return ghConn.SearchIssue(ctx, &req)
	}
}

func (s *Server) makeFetchHandler(op string) func(ctx context.Context, args json.RawMessage) (interface{}, error) {
	return func(ctx context.Context, args json.RawMessage) (interface{}, error) {
		var req struct {
			URL  string `json:"url"`
			Mode string `json:"mode"`
		}
		if err := json.Unmarshal(args, &req); err != nil {
			return nil, fmt.Errorf("invalid args: %w", err)
		}
		if req.Mode == "" {
			req.Mode = "auto"
		}
		switch op {
		case "fetch":
			return s.fetcherService.Fetch(ctx, req.URL)
		case "fetch_and_extract":
			return s.fetcherService.FetchAndExtract(ctx, req.URL, req.Mode)
		case "extract_structured":
			return s.fetcherService.ExtractStructured(ctx, req.URL)
		}
		return nil, fmt.Errorf("unknown fetch operation: %s", op)
	}
}

func (s *Server) makeValidateHandler(op string) func(ctx context.Context, args json.RawMessage) (interface{}, error) {
	return func(ctx context.Context, args json.RawMessage) (interface{}, error) {
		var req struct {
			URL  string   `json:"url"`
			URLs []string `json:"urls"`
		}
		if err := json.Unmarshal(args, &req); err != nil {
			return nil, fmt.Errorf("invalid args: %w", err)
		}
		switch op {
		case "validate_url":
			valid, err := s.fetcherService.ValidateURL(ctx, req.URL)
			var errMsg string
			if err != nil {
				errMsg = err.Error()
			}
			return map[string]interface{}{
				"url":   req.URL,
				"valid": valid,
				"error": errMsg,
			}, nil
		case "check_link_status":
			return s.fetcherService.CheckLinkStatus(ctx, req.URLs)
		}
		return nil, fmt.Errorf("unknown validate operation: %s", op)
	}
}

func (s *Server) stubSearchHandler(ctx context.Context, args json.RawMessage) (interface{}, error) {
	var req types.SearchRequest
	if err := json.Unmarshal(args, &req); err != nil {
		return nil, fmt.Errorf("invalid args: %w", err)
	}
	return &types.SearchResponse{
		Query:       req.Query,
		Results:     []types.SearchResultItem{},
		Summary:     "[STUB] Search not yet implemented",
		SourcesUsed:  []string{},
		Confidence:   0.0,
		Cached:       false,
		Warnings:     []string{"Stub implementation"},
	}, nil
}

func (s *Server) stubFetchHandler(ctx context.Context, args json.RawMessage) (interface{}, error) {
	var req struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(args, &req); err != nil {
		return nil, fmt.Errorf("invalid args: %w", err)
	}
	return &types.FetchResponse{
		URL:      req.URL,
		Title:    "[STUB] Title",
		Content:  "[STUB] Content",
		Metadata: map[string]string{},
		Warnings: []string{"Stub implementation"},
	}, nil
}

func (s *Server) stubValidateHandler(ctx context.Context, args json.RawMessage) (interface{}, error) {
	var req struct {
		URL  string   `json:"url"`
		URLs []string `json:"urls"`
	}
	if err := json.Unmarshal(args, &req); err != nil {
		return nil, fmt.Errorf("invalid args: %w", err)
	}
	if req.URL != "" {
		return map[string]interface{}{
			"url":     req.URL,
			"valid":   true,
			"warnings": []string{"Stub implementation"},
		}, nil
	}
	results := make([]map[string]interface{}, 0)
	for _, u := range req.URLs {
		results = append(results, map[string]interface{}{"url": u, "valid": true, "status": 200})
	}
	return map[string]interface{}{"results": results, "warnings": []string{"Stub implementation"}}, nil
}

func (s *Server) makeSynthesizeHandler(op string) func(ctx context.Context, args json.RawMessage) (interface{}, error) {
	return func(ctx context.Context, args json.RawMessage) (interface{}, error) {
		var req struct {
			Query   string                  `json:"query"`
			Results []types.SearchResultItem `json:"results"`
			Goal    string                  `json:"goal"`
			Style   string                  `json:"style"`
		}
		if err := json.Unmarshal(args, &req); err != nil {
			return nil, fmt.Errorf("invalid args: %w", err)
		}

		if s.synthesisService == nil {
			return map[string]interface{}{
				"summary":     "[STUB] Synthesis service not initialized",
				"keyFindings": []string{},
				"citations":   []string{},
				"query":       req.Query,
				"warnings":    []string{"Synthesis service not initialized"},
			}, nil
		}

		switch op {
		case "summarize":
			result, err := s.synthesisService.Summarize(ctx, req.Query, req.Results)
			if err != nil {
				return nil, err
			}
			return result, nil
		case "deep_research":
			result, err := s.synthesisService.DeepResearch(ctx, req.Query, req.Results)
			if err != nil {
				return nil, err
			}
			return result, nil
		case "compare_sources":
			result, err := s.synthesisService.CompareSources(ctx, req.Results, req.Query)
			if err != nil {
				return nil, err
			}
			return result, nil
		}
		return nil, fmt.Errorf("unknown synthesis operation: %s", op)
	}
}

func (s *Server) makeCacheHandler(op string) func(ctx context.Context, args json.RawMessage) (interface{}, error) {
	return func(ctx context.Context, args json.RawMessage) (interface{}, error) {
		var req struct {
			CacheKey string   `json:"cacheKey"`
			Query    string   `json:"query"`
			Sources  []string `json:"sources"`
			Limit    int      `json:"limit"`
			Offset   int      `json:"offset"`
		}
		if err := json.Unmarshal(args, &req); err != nil {
			return nil, fmt.Errorf("invalid args: %w", err)
		}

		switch op {
		case "get":
			if s.cacheService == nil {
				return map[string]interface{}{"cacheKey": req.CacheKey, "hit": false, "warnings": []string{"Cache service not initialized"}}, nil
			}
			result, hit, err := s.cacheService.GetCached(ctx, req.Query, req.Sources)
			if err != nil {
				return nil, err
			}
			if hit && result != nil {
				return result, nil
			}
			return map[string]interface{}{
				"cacheKey": req.CacheKey,
				"hit":      hit,
				"query":    req.Query,
				"sources":  req.Sources,
				"warnings": []string{},
			}, nil

		case "invalidate":
			if s.cacheService == nil {
				return map[string]interface{}{"cacheKey": req.CacheKey, "invalidated": false, "warnings": []string{"Cache service not initialized"}}, nil
			}
			err := s.cacheService.InvalidateCache(ctx, req.Query, req.Sources)
			if err != nil {
				return nil, err
			}
			return map[string]interface{}{
				"cacheKey":   req.CacheKey,
				"invalidated": true,
				"query":      req.Query,
				"sources":    req.Sources,
			}, nil

		case "history":
			if s.historyService == nil {
				return map[string]interface{}{"history": []interface{}{}, "warnings": []string{"History service not initialized"}}, nil
			}
			limit := req.Limit
			if limit <= 0 {
				limit = 20
			}
			offset := req.Offset
			if offset < 0 {
				offset = 0
			}
			history, err := s.historyService.GetSearchHistory(ctx, limit, offset)
			if err != nil {
				return nil, err
			}
			return map[string]interface{}{
				"history": history,
				"limit":   limit,
				"offset":  offset,
			}, nil
		}
		return nil, fmt.Errorf("unknown cache operation: %s", op)
	}
}

func (s *Server) getCurrentDateHandler(ctx context.Context, args json.RawMessage) (interface{}, error) {
	now := time.Now().UTC()
	return map[string]interface{}{
		"date":      now.Format("2006-01-02"),
		"time":      now.Format("15:04:05"),
		"timezone":  "UTC",
		"timestamp": now.Unix(),
	}, nil
}

func (s *Server) ListTools() []map[string]interface{} {
	tools := make([]map[string]interface{}, 0, len(s.toolsRegistry))
	for _, t := range s.toolsRegistry {
		tools = append(tools, map[string]interface{}{
			"name":        t.Name,
			"description": t.Description,
			"inputSchema": t.InputSchema,
		})
	}
	return tools
}
