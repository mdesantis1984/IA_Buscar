package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/thiscloud/ia-buscar/internal/auth"
	"github.com/thiscloud/ia-buscar/internal/cache"
	"github.com/thiscloud/ia-buscar/internal/fetch"
	"github.com/thiscloud/ia-buscar/internal/observability"
	"github.com/thiscloud/ia-buscar/internal/search"
	"github.com/thiscloud/ia-buscar/internal/synthesis"
)

type Server struct {
	transport        string
	httpAddr         string
	searxngURL       string
	cacheTTL         int
	memoryURL        string
	memoryAPIKey     string
	toolsRegistry    []Tool
	connectorManager *search.ConnectorManager
	planner          *search.Planner
	met              *observability.Metrics
	fetcherService   *fetch.FetcherService
	synthesisService *synthesis.Service
	cacheService     *cache.Service
	historyService   *cache.HistoryService
	authValidator    *auth.Validator
}

type Tool struct {
	Name        string
	Description string
	InputSchema map[string]interface{}
	Handler     func(ctx context.Context, args json.RawMessage) (interface{}, error)
}

func NewServer(connectorManager *search.ConnectorManager, planner *search.Planner, transport, httpAddr, searxngURL string, cacheTTL int, memoryURL, memoryAPIKey string, fetchTimeoutMs int, fetchSvc *fetch.FetcherService, synthSvc *synthesis.Service, cacheSvc *cache.Service, historySvc *cache.HistoryService, authValidator *auth.Validator) *Server {
	s := &Server{
		transport:        transport,
		httpAddr:         httpAddr,
		searxngURL:       searxngURL,
		cacheTTL:         cacheTTL,
		memoryURL:        memoryURL,
		memoryAPIKey:     memoryAPIKey,
		connectorManager: connectorManager,
		planner:          planner,
		met:              observability.New(),
		fetcherService:   fetchSvc,
		synthesisService: synthSvc,
		cacheService:     cacheSvc,
		historyService:   historySvc,
		authValidator:    authValidator,
	}
	s.buildToolsRegistry()
	s.registerTools()
	return s
}

func (s *Server) buildToolsRegistry() {
	s.toolsRegistry = []Tool{
		{Name: "search_web", Description: "Búsqueda web amplia", InputSchema: searchInputSchema()},
		{Name: "search_news", Description: "Noticias y actualidad", InputSchema: searchInputSchema()},
		{Name: "search_doc_oficial", Description: "Documentación oficial de productos, frameworks o servicios", InputSchema: searchInputSchema()},
		{Name: "search_local_index", Description: "Índice local de workspace o fuentes indexadas", InputSchema: searchInputSchema()},
		{Name: "search_github", Description: "Repositorios, archivos y commits", InputSchema: searchInputSchema()},
		{Name: "search_github_pr", Description: "Pull requests", InputSchema: searchInputSchema()},
		{Name: "search_github_issue", Description: "Issues", InputSchema: searchInputSchema()},
		{Name: "search_stackoverflow", Description: "Q&A técnica", InputSchema: searchInputSchema()},
		{Name: "search_npm", Description: "Paquetes Node/TS", InputSchema: searchInputSchema()},
		{Name: "search_nuget", Description: "Paquetes .NET", InputSchema: searchInputSchema()},
		{Name: "search_pypi", Description: "Paquetes Python", InputSchema: searchInputSchema()},
		{Name: "search_docker_hub", Description: "Imágenes Docker", InputSchema: searchInputSchema()},
		{Name: "search_academic", Description: "Papers, preprints y referencias académicas", InputSchema: searchInputSchema()},
		{Name: "search_reddit", Description: "Discusiones y experiencias reales", InputSchema: searchInputSchema()},
		{Name: "search_youtube", Description: "Tutoriales y demos", InputSchema: searchInputSchema()},
		{Name: "search_images", Description: "Diagramas, capturas o material visual", InputSchema: searchInputSchema()},
		{Name: "fetch_url", Description: "Obtener una URL", InputSchema: fetchURLInputSchema()},
		{Name: "fetch_and_extract", Description: "Extraer contenido útil", InputSchema: fetchURLInputSchema()},
		{Name: "extract_structured", Description: "Extraer tablas, listas, metadatos o estructura", InputSchema: fetchURLInputSchema()},
		{Name: "validate_url", Description: "Verificar accesibilidad y seguridad de URL", InputSchema: urlInputSchema()},
		{Name: "check_link_status", Description: "Validar lote de enlaces", InputSchema: urlListInputSchema()},
		{Name: "summarize_results", Description: "Síntesis breve", InputSchema: synthesisInputSchema()},
		{Name: "deep_research", Description: "Síntesis consolidada de múltiples fuentes", InputSchema: synthesisInputSchema()},
		{Name: "compare_sources", Description: "Comparar resultados o explicaciones", InputSchema: synthesisInputSchema()},
		{Name: "get_cached", Description: "Recuperar caché", InputSchema: cacheKeyInputSchema()},
		{Name: "invalidate_cache", Description: "Borrar caché", InputSchema: cacheKeyInputSchema()},
		{Name: "get_search_history", Description: "Historial de búsquedas", InputSchema: historyInputSchema()},
		{Name: "get_current_date", Description: "Fecha/hora consistente para contexto", InputSchema: emptyInputSchema()},
	}
}

func searchInputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"query":      map[string]interface{}{"type": "string", "description": "Texto de búsqueda"},
			"sources":    map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "description": "Fuentes a consultar"},
			"maxResults": map[string]interface{}{"type": "integer", "description": "Máximo de resultados"},
			"language":   map[string]interface{}{"type": "string", "description": "Código de idioma (ej: en, es)"},
			"safeSearch": map[string]interface{}{"type": "boolean", "description": "Filtrar contenido seguro"},
			"timeRange":  map[string]interface{}{"type": "string", "description": "Rango temporal (day, week, month, year)"},
		},
		"required": []string{"query"},
	}
}

func fetchURLInputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"url":       map[string]interface{}{"type": "string", "description": "URL a obtener"},
			"mode":      map[string]interface{}{"type": "string", "description": "Modo de extracción"},
			"timeoutMs": map[string]interface{}{"type": "integer", "description": "Timeout en milisegundos"},
		},
		"required": []string{"url"},
	}
}

func urlInputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"url": map[string]interface{}{"type": "string", "description": "URL a validar"},
		},
		"required": []string{"url"},
	}
}

func urlListInputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"urls": map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "description": "Lista de URLs"},
		},
		"required": []string{"urls"},
	}
}

func synthesisInputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"query":   map[string]interface{}{"type": "string", "description": "Consulta original"},
			"results": map[string]interface{}{"type": "array", "description": "Resultados a sintetizar"},
			"goal":    map[string]interface{}{"type": "string", "description": "Objetivo de la síntesis"},
			"style":   map[string]interface{}{"type": "string", "description": "Estilo de síntesis"},
		},
		"required": []string{"query"},
	}
}

func cacheKeyInputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"cacheKey": map[string]interface{}{"type": "string", "description": "Clave de caché"},
		},
	}
}

func historyInputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"limit":  map[string]interface{}{"type": "integer", "description": "Límite de resultados"},
			"offset": map[string]interface{}{"type": "integer", "description": "Offset de paginación"},
		},
	}
}

func emptyInputSchema() map[string]interface{} {
	return map[string]interface{}{"type": "object", "properties": map[string]interface{}{}}
}

func (s *Server) Tools() []Tool {
	return s.toolsRegistry
}

func (s *Server) HandleInitialize(ctx context.Context, params json.RawMessage) (interface{}, error) {
	var req struct {
		ClientID          string `json:"clientId"`
		ProtocolVersion   string `json:"protocolVersion"`
		ClientCapabilities map[string]interface{} `json:"clientCapabilities"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	if req.ClientID == "" {
		req.ClientID = "anonymous-" + uuid.New().String()[:8]
	}
	sessionID := uuid.New().String()
	return map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"serverInfo": map[string]interface{}{
			"name":    "ia-buscar",
			"version": "1.0.0",
		},
		"capabilities": map[string]interface{}{
			"tools": map[string]interface{}{"listChanged": false},
		},
		"sessionId": sessionID,
	}, nil
}

func (s *Server) HandleToolsList(ctx context.Context, params json.RawMessage) (interface{}, error) {
	var req struct {
		Filter struct {
			Capability string `json:"capability"`
			Tag        string `json:"tag"`
		} `json:"filter"`
	}
	_ = json.Unmarshal(params, &req)
	tools := make([]map[string]interface{}, 0, len(s.toolsRegistry))
	for _, t := range s.toolsRegistry {
		tools = append(tools, map[string]interface{}{
			"name":        t.Name,
			"description": t.Description,
			"inputSchema": t.InputSchema,
		})
	}
	return map[string]interface{}{"tools": tools}, nil
}

func (s *Server) HandleToolsCall(ctx context.Context, params json.RawMessage) (interface{}, error) {
	var req struct {
		Name      string                 `json:"name"`
		Arguments map[string]interface{} `json:"arguments"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	for _, t := range s.toolsRegistry {
		if t.Name == req.Name {
			inputJSON, _ := json.Marshal(req.Arguments)
			result, err := t.Handler(ctx, inputJSON)
			if err != nil {
				return nil, err
			}
			s.met.IncrToolCall(t.Name)
			return map[string]interface{}{
				"content": []map[string]interface{}{
					{"type": "text", "text": formatResult(result)},
				},
			}, nil
		}
	}
	return nil, fmt.Errorf("tool not found: %s", req.Name)
}

func formatResult(v interface{}) string {
	b, _ := json.MarshalIndent(v, "", "  ")
	return string(b)
}

func (s *Server) HandleHTTP(w http.ResponseWriter, r *http.Request) {
	setCORSHeaders(w, r)
	switch r.Method {
	case http.MethodGet:
		s.handleHTTPGet(w, r)
	case http.MethodPost:
		s.handleHTTPPost(w, r)
	case http.MethodOptions:
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleHTTPGet(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	if sid := r.Header.Get("Mcp-Session-Id"); sid != "" {
		w.Header().Set("Mcp-Session-Id", sid)
	}
	w.WriteHeader(http.StatusOK)
	flusher, ok := w.(http.Flusher)
	if !ok {
		return
	}
	fmt.Fprintf(w, ": ping\n\n")
	flusher.Flush()
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			fmt.Fprintf(w, ": ping\n\n")
			flusher.Flush()
		}
	}
}

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Method  string           `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

func (s *Server) handleHTTPPost(w http.ResponseWriter, r *http.Request) {
	var req rpcRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"jsonrpc": "2.0",
			"error":   map[string]interface{}{"code": -32700, "message": "parse error"},
		})
		return
	}
	isNotification := req.ID == nil
	var resp interface{}
	switch req.Method {
	case "initialize":
		resp = s.handleMCPInitialize(req.ID)
	case "tools/list", "mcp.tools.list":
		resp = s.handleMCPToolsList(req.ID)
	case "tools/call", "mcp.tools.call":
		resp = s.handleMCPToolsCall(r.Context(), req.ID, req.Params)
	case "ping":
		resp = map[string]interface{}{"jsonrpc": "2.0", "id": req.ID, "result": map[string]interface{}{}}
	default:
		if !isNotification {
			resp = map[string]interface{}{
				"jsonrpc": "2.0",
				"id":     req.ID,
				"error":  map[string]interface{}{"code": -32601, "message": fmt.Sprintf("method not found: %s", req.Method)},
			}
		}
	}
	if isNotification {
		w.WriteHeader(http.StatusAccepted)
		return
	}
	code := http.StatusOK
	writeJSON(w, code, resp)
}

func (s *Server) handleMCPInitialize(id interface{}) map[string]interface{} {
	return map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"result": map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"serverInfo":     map[string]interface{}{"name": "ia-buscar", "version": "1.0.0"},
			"capabilities":   map[string]interface{}{"tools": map[string]interface{}{"listChanged": false}},
		},
	}
}

func (s *Server) handleMCPToolsList(id interface{}) map[string]interface{} {
	tools := make([]map[string]interface{}, 0, len(s.toolsRegistry))
	for _, t := range s.toolsRegistry {
		tools = append(tools, map[string]interface{}{
			"name":        t.Name,
			"description": t.Description,
			"inputSchema": t.InputSchema,
		})
	}
	return map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"result":  map[string]interface{}{"tools": tools},
	}
}

func (s *Server) handleMCPToolsCall(ctx context.Context, id interface{}, params json.RawMessage) map[string]interface{} {
	var reqParams struct {
		Name      string                 `json:"name"`
		Arguments map[string]interface{} `json:"arguments"`
	}
	if err := json.Unmarshal(params, &reqParams); err != nil {
		return map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      id,
			"error":   map[string]interface{}{"code": -32602, "message": "invalid params: " + err.Error()},
		}
	}
	if reqParams.Name == "" {
		return map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      id,
			"error":   map[string]interface{}{"code": -32602, "message": "name is required"},
		}
	}
	for _, t := range s.toolsRegistry {
		if t.Name == reqParams.Name {
			inputJSON, _ := json.Marshal(reqParams.Arguments)
			result, err := t.Handler(ctx, inputJSON)
			if err != nil {
				return map[string]interface{}{
					"jsonrpc": "2.0",
					"id":      id,
					"error":   map[string]interface{}{"code": -32603, "message": err.Error()},
				}
			}
			s.met.IncrToolCall(t.Name)
			return map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      id,
				"result": map[string]interface{}{
					"content": []map[string]interface{}{
						{"type": "text", "text": formatResult(result)},
					},
				},
			}
		}
	}
	return map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"error":   map[string]interface{}{"code": -32602, "message": fmt.Sprintf("tool not found: %s", reqParams.Name)},
	}
}

func writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func setCORSHeaders(w http.ResponseWriter, r *http.Request) {
	origin := r.Header.Get("Origin")
	if origin == "" {
		origin = "*"
	}
	w.Header().Set("Access-Control-Allow-Origin", origin)
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Api-Key, Mcp-Session-Id, Accept")
	w.Header().Set("Access-Control-Max-Age", "86400")
}

func (s *Server) Start(ctx context.Context) error {
	if s.transport == "http" {
		mux := http.NewServeMux()

		var handler http.Handler
		handler = mux

		if s.authValidator != nil {
			handler = s.authValidator.Middleware(handler)
		}

		mux.HandleFunc("/mcp", s.HandleHTTP)
		mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, http.StatusOK, map[string]interface{}{"status": "ok"})
		})
		mux.HandleFunc("/metrics", s.met.Handler())
		srv := &http.Server{Addr: s.httpAddr, Handler: handler}
		go srv.ListenAndServe()
		return nil
	}
	return nil
}

func (s *Server) Stop(ctx context.Context) error {
	return nil
}

func (s *Server) Name() string {
	return s.transport
}

