package observability

import (
	"encoding/json"
	"net/http"
	"sync"
	"sync/atomic"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Metrics struct {
	mu             sync.RWMutex
	toolsCalls     map[string]int64
	sessionsActive int64
	searchesTotal  int64
	errorsTotal    int64
	registry       *prometheus.Registry
	httpRequests   *prometheus.CounterVec
	searchLatency  *prometheus.HistogramVec
}

func New() *Metrics {
	m := &Metrics{
		toolsCalls: make(map[string]int64),
		registry:   prometheus.NewRegistry(),
	}
	m.registry.MustRegister(prometheus.NewGoCollector())
	m.registry.MustRegister(prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))
	m.httpRequests = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ia_buscar_http_requests_total",
			Help: "Total HTTP requests",
		},
		[]string{"method", "path", "status"},
	)
	m.searchLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "ia_buscar_search_latency_seconds",
			Help:    "Search latency",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"source"},
	)
	m.registry.MustRegister(m.httpRequests)
	m.registry.MustRegister(m.searchLatency)
	return m
}

func (m *Metrics) IncrToolCall(toolName string) {
	m.mu.Lock()
	m.toolsCalls[toolName]++
	m.mu.Unlock()
}

func (m *Metrics) IncrSession() {
	atomic.AddInt64(&m.sessionsActive, 1)
}

func (m *Metrics) IncrSearch(source string) {
	atomic.AddInt64(&m.searchesTotal, 1)
}

func (m *Metrics) IncrError() {
	atomic.AddInt64(&m.errorsTotal, 1)
}

func (m *Metrics) SetSkillsLoaded(int64) {}

func (m *Metrics) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h := promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
		h.ServeHTTP(w, r)
	}
}

func (m *Metrics) JSON() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	data := map[string]interface{}{
		"toolsCalls":     m.toolsCalls,
		"sessionsActive": atomic.LoadInt64(&m.sessionsActive),
		"searchesTotal":  atomic.LoadInt64(&m.searchesTotal),
		"errorsTotal":    atomic.LoadInt64(&m.errorsTotal),
	}
	b, _ := json.MarshalIndent(data, "", "  ")
	return string(b)
}

func (m *Metrics) RecordSearchLatency(source string, seconds float64) {
	m.searchLatency.WithLabelValues(source).Observe(seconds)
}

func (m *Metrics) RecordHTTPRequest(method, path, status string) {
	m.httpRequests.WithLabelValues(method, path, status).Inc()
}