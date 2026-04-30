package search

import (
	"context"
	"encoding/json"

	"github.com/thiscloud/ia-buscar/pkg/types"
)

type Planner struct{}

func NewPlanner() *Planner {
	return &Planner{}
}

func (p *Planner) Plan(ctx context.Context, req *types.SearchRequest) (*Plan, error) {
	plan := &Plan{
		Query:      req.Query,
		Strategies: []string{},
	}
	return plan, nil
}

type Plan struct {
	Query      string
	Strategies []string
	Sources    []string
	MaxResults int
}

func (p *Planner) ClassifyIntent(ctx context.Context, query string) (string, error) {
	return "general", nil
}

func (p *Planner) SelectSources(ctx context.Context, intent string) ([]string, error) {
	switch intent {
	case "github":
		return []string{"github"}, nil
	case "npm":
		return []string{"npm"}, nil
	case "nuget":
		return []string{"nuget"}, nil
	case "pypi":
		return []string{"pypi"}, nil
	case "stackoverflow":
		return []string{"stackoverflow"}, nil
	default:
		return []string{"web"}, nil
	}
}

func classifyQueryJSON(query string) string {
	data, _ := json.Marshal(map[string]string{"query": query})
	return string(data)
}
