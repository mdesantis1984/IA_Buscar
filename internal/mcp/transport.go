package mcp

import (
	"context"
)

type Transport interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Name() string
}

type STDIOTransport struct {
	server *Server
}

func NewSTDIOTransport(s *Server) *STDIOTransport {
	return &STDIOTransport{server: s}
}

func (t *STDIOTransport) Name() string { return "stdio" }

func (t *STDIOTransport) Start(ctx context.Context) error {
	return nil
}

func (t *STDIOTransport) Stop(ctx context.Context) error {
	return nil
}

type HTTPTransport struct {
	addr   string
	server *Server
}

func NewHTTPTransport(addr string, s *Server) *HTTPTransport {
	return &HTTPTransport{addr: addr, server: s}
}

func (t *HTTPTransport) Name() string { return "http" }

func (t *HTTPTransport) Start(ctx context.Context) error {
	return t.server.Start(ctx)
}

func (t *HTTPTransport) Stop(ctx context.Context) error {
	return t.server.Stop(ctx)
}