package search

import (
	"context"

	"github.com/thiscloud/ia-buscar/pkg/types"
)

type Service struct {
	searxngURL string
}

func NewService(searxngURL string) *Service {
	return &Service{searxngURL: searxngURL}
}

func (s *Service) Search(ctx context.Context, req *types.SearchRequest) (*types.SearchResponse, error) {
	return &types.SearchResponse{
		Query:     req.Query,
		Results:   []types.SearchResultItem{},
		Cached:    false,
		Warnings:  []string{"stub implementation"},
	}, nil
}

func (s *Service) SearchGithub(ctx context.Context, req *types.SearchRequest) (*types.SearchResponse, error) {
	return s.Search(ctx, req)
}

func (s *Service) SearchNPM(ctx context.Context, req *types.SearchRequest) (*types.SearchResponse, error) {
	return s.Search(ctx, req)
}

func (s *Service) SearchNuGet(ctx context.Context, req *types.SearchRequest) (*types.SearchResponse, error) {
	return s.Search(ctx, req)
}

func (s *Service) SearchPyPI(ctx context.Context, req *types.SearchRequest) (*types.SearchResponse, error) {
	return s.Search(ctx, req)
}

func (s *Service) SearchStackOverflow(ctx context.Context, req *types.SearchRequest) (*types.SearchResponse, error) {
	return s.Search(ctx, req)
}
