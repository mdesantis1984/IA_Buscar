package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"sync"
	"time"

	"github.com/thiscloud/ia-buscar/pkg/types"
)

type Service struct {
	entries map[string]*entry
	mu      sync.RWMutex
	ttl     time.Duration
}

type entry struct {
	key      string
	value    []byte
	created  time.Time
	expires  time.Time
	sources  []string
}

func NewService(ttlSeconds int) *Service {
	return &Service{
		entries: make(map[string]*entry),
		ttl:     time.Duration(ttlSeconds) * time.Second,
	}
}

func (s *Service) Get(ctx context.Context, cacheKey string) (*types.CacheEntry, bool, error) {
	s.mu.RLock()
	e, ok := s.entries[cacheKey]
	s.mu.RUnlock()
	if !ok {
		return nil, false, nil
	}
	if time.Now().After(e.expires) {
		s.Delete(ctx, cacheKey)
		return nil, false, nil
	}
	return &types.CacheEntry{
		CacheKey:  e.key,
		CreatedAt: e.created,
		ExpiresAt: e.expires,
		Payload:   e.value,
		SourceSet: e.sources,
	}, true, nil
}

func (s *Service) Set(ctx context.Context, cacheKey string, payload []byte, sources []string) error {
	now := time.Now()
	s.mu.Lock()
	s.entries[cacheKey] = &entry{
		key:     cacheKey,
		value:   payload,
		created: now,
		expires: now.Add(s.ttl),
		sources: sources,
	}
	s.mu.Unlock()
	return nil
}

func (s *Service) Delete(ctx context.Context, cacheKey string) error {
	s.mu.Lock()
	delete(s.entries, cacheKey)
	s.mu.Unlock()
	return nil
}

func (s *Service) Clear(ctx context.Context) error {
	s.mu.Lock()
	s.entries = make(map[string]*entry)
	s.mu.Unlock()
	return nil
}

func (s *Service) Keys(ctx context.Context) ([]string, error) {
	s.mu.RLock()
	keys := make([]string, 0, len(s.entries))
	for k := range s.entries {
		keys = append(keys, k)
	}
	s.mu.RUnlock()
	return keys, nil
}

func GenerateCacheKey(query string, sources []string) string {
	h := sha256.New()
	h.Write([]byte(query))
	for _, s := range sources {
		h.Write([]byte(s))
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

func (s *Service) History(ctx context.Context, limit, offset int) ([]*types.CacheEntry, error) {
	s.mu.RLock()
	entries := make([]*types.CacheEntry, 0)
	for _, e := range s.entries {
		entries = append(entries, &types.CacheEntry{
			CacheKey:  e.key,
			CreatedAt: e.created,
			ExpiresAt: e.expires,
			SourceSet: e.sources,
		})
	}
	s.mu.RUnlock()
	if offset >= len(entries) {
		return []*types.CacheEntry{}, nil
	}
	end := offset + limit
	if end > len(entries) {
		end = len(entries)
	}
	return entries[offset:end], nil
}

func (s *Service) GetCached(ctx context.Context, query string, sources []string) (*types.SearchResponse, bool, error) {
	cacheKey := GenerateCacheKey(query, sources)

	s.mu.RLock()
	e, ok := s.entries[cacheKey]
	s.mu.RUnlock()

	if !ok {
		return nil, false, nil
	}

	if time.Now().After(e.expires) {
		s.Delete(ctx, cacheKey)
		return nil, false, nil
	}

	var response types.SearchResponse
	if err := json.Unmarshal(e.value, &response); err != nil {
		return nil, false, err
	}

	response.Cached = true
	return &response, true, nil
}

func (s *Service) InvalidateCache(ctx context.Context, query string, sources []string) error {
	if query == "*" && len(sources) == 0 {
		return s.Clear(ctx)
	}

	if len(sources) == 0 {
		s.mu.Lock()
		keysToDelete := make([]string, 0)
		for k, e := range s.entries {
			if stringsContains(e.sources, query) || k == query {
				keysToDelete = append(keysToDelete, k)
			}
		}
		for _, k := range keysToDelete {
			delete(s.entries, k)
		}
		s.mu.Unlock()
		return nil
	}

	cacheKey := GenerateCacheKey(query, sources)
	return s.Delete(ctx, cacheKey)
}

func stringsContains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

type HistoryService struct {
	entries     []SearchHistoryEntry
	mu          sync.RWMutex
	maxEntries  int
	persistPath string
	persistCnt  int
	persistMux  sync.Mutex
}

type SearchHistoryEntry struct {
	Timestamp   time.Time `json:"timestamp"`
	Query       string    `json:"query"`
	Sources     []string  `json:"sources"`
	ResultsCount int      `json:"resultsCount"`
	LatencyMs   int64    `json:"latencyMs"`
	Cached      bool      `json:"cached"`
}

func NewHistoryService(persistPath string) *HistoryService {
	hs := &HistoryService{
		entries:     make([]SearchHistoryEntry, 0),
		maxEntries:  1000,
		persistPath: persistPath,
		persistCnt:  0,
	}
	hs.loadFromDisk()
	return hs
}

func (s *HistoryService) AddEntry(ctx context.Context, entry SearchHistoryEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.entries) >= s.maxEntries {
		s.entries = s.entries[1:]
	}

	entry.Timestamp = time.Now().UTC()
	s.entries = append(s.entries, entry)

	s.persistCnt++
	if s.persistCnt >= 10 && s.persistPath != "" {
		go s.persistToDisk()
		s.persistCnt = 0
	}

	return nil
}

func (s *HistoryService) GetSearchHistory(ctx context.Context, limit int, offset int) ([]SearchHistoryEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if offset >= len(s.entries) {
		return []SearchHistoryEntry{}, nil
	}

	end := offset + limit
	if end > len(s.entries) {
		end = len(s.entries)
	}

	result := make([]SearchHistoryEntry, end-offset)
	copy(result, s.entries[offset:end])
	return result, nil
}

func (s *HistoryService) ClearHistory(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries = make([]SearchHistoryEntry, 0)
	return nil
}

func (s *HistoryService) persistToDisk() {
	s.persistMux.Lock()
	defer s.persistMux.Unlock()

	if s.persistPath == "" {
		return
	}

	data, err := json.Marshal(s.entries)
	if err != nil {
		return
	}

	tmpPath := s.persistPath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return
	}
	os.Rename(tmpPath, s.persistPath)
}

func (s *HistoryService) loadFromDisk() {
	if s.persistPath == "" {
		return
	}

	data, err := os.ReadFile(s.persistPath)
	if err != nil {
		return
	}

	var entries []SearchHistoryEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return
	}

	s.mu.Lock()
	s.entries = entries
	if len(s.entries) > s.maxEntries {
		s.entries = s.entries[len(s.entries)-s.maxEntries:]
	}
	s.mu.Unlock()
}

func GetCurrentDate() string {
	return time.Now().UTC().Format(time.RFC3339)
}
