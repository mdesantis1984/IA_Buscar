package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/thiscloud/ia-buscar/internal/auth"
	"github.com/thiscloud/ia-buscar/internal/cache"
	"github.com/thiscloud/ia-buscar/internal/connectors"
	"github.com/thiscloud/ia-buscar/internal/fetch"
	"github.com/thiscloud/ia-buscar/internal/memory"
	"github.com/thiscloud/ia-buscar/internal/mcp"
	"github.com/thiscloud/ia-buscar/internal/observability"
	"github.com/thiscloud/ia-buscar/internal/search"
	"github.com/thiscloud/ia-buscar/internal/synthesis"
)

var (
	transport       = flag.String("transport", "stdio", "Transport mode: stdio or http")
	httpAddr        = flag.String("http-addr", ":8080", "HTTP server address")
	searxngURL      = flag.String("searxng-url", "http://10.0.0.201:8080", "SearxNG URL")
	cacheTTL        = flag.Int("cache-ttl", 300, "Cache TTL in seconds")
	memoryURL       = flag.String("memory-url", "http://127.0.0.1:7438", "IA_Recuerdo service URL")
	memoryKey       = flag.String("memory-apikey", "", "IA_Recuerdo API key")
	fetchTimeoutMs  = flag.Int("fetch-timeout-ms", 30000, "Fetch timeout in milliseconds")
	authKey         = flag.String("auth-key", "", "API key for authentication (optional)")
)

func main() {
	flag.Parse()
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("=== IA_Buscar Starting ===")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cacheSvc := cache.NewService(*cacheTTL)
	historySvc := cache.NewHistoryService("")
	searchSvc := search.NewService(*searxngURL)
	fetchSvc := fetch.NewFetcherService(30000)
	synthSvc := synthesis.NewService()
	memClient := memory.NewClient(*memoryURL, "ia-buscar", *memoryKey)
	_ = observability.New()
	observability.InitTracing("ia-buscar")
	authValidator := auth.NewValidator(*authKey)

	cm := search.NewConnectorManager(cacheSvc, memClient)
	planner := search.NewPlanner()
	cm.Register(connectors.NewWebConnector(*searxngURL, cacheSvc, memClient))
	cm.Register(connectors.NewGitHubConnector("", cacheSvc, memClient))
	cm.Register(connectors.NewStackOverflowConnector(cacheSvc, memClient))
	cm.Register(connectors.NewNPMConnector(cacheSvc, memClient))
	cm.Register(connectors.NewNuGetConnector(cacheSvc, memClient))
	cm.Register(connectors.NewPyPIConnector(cacheSvc, memClient))
	cm.Register(connectors.NewDockerHubConnector(cacheSvc, memClient))
	cm.Register(connectors.NewAcademicConnector(*searxngURL, cacheSvc, memClient))
	cm.Register(connectors.NewRedditConnector(cacheSvc, memClient))
	cm.Register(connectors.NewYouTubeConnector(*searxngURL, cacheSvc, memClient))
	cm.Register(connectors.NewImagesConnector(*searxngURL, cacheSvc, memClient))
	cm.Register(connectors.NewNewsConnector(*searxngURL, cacheSvc, memClient))

	server := mcp.NewServer(cm, planner, *transport, *httpAddr, *searxngURL, *cacheTTL, *memoryURL, *memoryKey, *fetchTimeoutMs, fetchSvc, synthSvc, cacheSvc, historySvc, authValidator)
	_ = searchSvc
	_ = memClient
	var trans mcp.Transport
	switch *transport {
	case "stdio":
		trans = mcp.NewSTDIOTransport(server)
	case "http":
		trans = mcp.NewHTTPTransport(*httpAddr, server)
	default:
		log.Fatalf("Unknown transport mode: %s", *transport)
	}
	if err := trans.Start(ctx); err != nil {
		log.Fatalf("Failed to start transport: %v", err)
	}
	log.Printf("IA_Buscar running with %s transport", trans.Name())
	memClient.Save(ctx, &memory.Observation{
		Title:   "IA_Buscar started",
		Content: fmt.Sprintf("**Transport**: %s\n**SearxNG**: %s", trans.Name(), *searxngURL),
		Type:    "discovery",
	})
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan
	log.Println("Shutdown signal received")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := trans.Stop(shutdownCtx); err != nil {
		log.Printf("Transport shutdown error: %v", err)
	}
	log.Println("=== IA_Buscar Stopped ===")
}