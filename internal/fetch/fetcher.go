package fetch

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/thiscloud/ia-buscar/pkg/types"
)

type FetcherService struct {
	client    *http.Client
	extractor *Extractor
	timeoutMs int
}

func NewFetcherService(timeoutMs int) *FetcherService {
	return &FetcherService{
		client: &http.Client{
			Timeout: time.Duration(timeoutMs) * time.Millisecond,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		extractor: NewExtractor(),
		timeoutMs: timeoutMs,
	}
}

var privateIPBlocks = []*net.IPNet{
	parseCIDR("10.0.0.0/8"),
	parseCIDR("172.16.0.0/12"),
	parseCIDR("192.168.0.0/16"),
	parseCIDR("127.0.0.0/8"),
	parseCIDR("169.254.0.0/16"),
	parseCIDR("0.0.0.0/8"),
}

func parseCIDR(cidr string) *net.IPNet {
	_, ipNet, _ := net.ParseCIDR(cidr)
	return ipNet
}

func isPrivateIP(ip net.IP) bool {
	for _, block := range privateIPBlocks {
		if block.Contains(ip) {
			return true
		}
	}
	return false
}

var localhostPatterns = regexp.MustCompile(`(?i)(localhost|loopback|local|internal|\.local$|\.internal$|broadcasthost)`)

func (s *FetcherService) isAllowedURL(rawURL string) error {
	if rawURL == "" {
		return fmt.Errorf("URL cannot be empty")
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL format: %w", err)
	}

	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("unsupported scheme: %s (only http and https allowed)", parsed.Scheme)
	}

	if parsed.Host == "" {
		return fmt.Errorf("URL must have a host")
	}

	host := strings.ToLower(parsed.Host)
	if localhostPatterns.MatchString(host) {
		return fmt.Errorf("URL host %s is not allowed (internal host)", host)
	}

	if ip := net.ParseIP(host); ip != nil {
		if isPrivateIP(ip) {
			return fmt.Errorf("private IP addresses are not allowed: %s", host)
		}
	}

	return nil
}

func (s *FetcherService) Fetch(ctx context.Context, rawURL string) (*types.FetchResponse, error) {
	if err := s.isAllowedURL(rawURL); err != nil {
		return &types.FetchResponse{
			URL:      rawURL,
			Warnings: []string{err.Error()},
		}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; IA-Buscar/1.0; +https://thiscloud.es)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")

	resp, err := s.client.Do(req)
	if err != nil {
		return &types.FetchResponse{
			URL:      rawURL,
			Warnings: []string{fmt.Sprintf("request failed: %v", err)},
		}, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 50*1024*1024))
	if err != nil {
		return &types.FetchResponse{
			URL:      rawURL,
			Warnings: []string{"failed to read response body"},
		}, fmt.Errorf("failed to read response: %w", err)
	}

	contentType := resp.Header.Get("Content-Type")
	isHTML := strings.Contains(strings.ToLower(contentType), "text/html")

	result := &types.FetchResponse{
		URL:      rawURL,
		Metadata: map[string]string{},
	}

	if isHTML {
		extractResp, err := s.extractor.Extract(ctx, string(body), rawURL)
		if err == nil {
			result.Title = extractResp.Title
			result.Content = extractResp.Content
			result.Metadata = extractResp.Metadata
		} else {
			result.Content = string(body)
			result.Warnings = append(result.Warnings, fmt.Sprintf("extraction failed: %v", err))
		}
	} else {
		result.Content = string(body)
	}

	if result.Title == "" {
		result.Title = rawURL
	}

	return result, nil
}

func (s *FetcherService) FetchAndExtract(ctx context.Context, rawURL string, mode string) (*types.FetchResponse, error) {
	if err := s.isAllowedURL(rawURL); err != nil {
		return &types.FetchResponse{
			URL:      rawURL,
			Warnings: []string{err.Error()},
		}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; IA-Buscar/1.0; +https://thiscloud.es)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")

	resp, err := s.client.Do(req)
	if err != nil {
		return &types.FetchResponse{
			URL:      rawURL,
			Warnings: []string{fmt.Sprintf("request failed: %v", err)},
		}, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 50*1024*1024))
	if err != nil {
		return &types.FetchResponse{
			URL:      rawURL,
			Warnings: []string{"failed to read response body"},
		}, fmt.Errorf("failed to read response: %w", err)
	}

	html := string(body)
	var extractedContent string

	switch mode {
	case "article":
		extractedContent, err = s.extractor.ExtractArticleContent(html)
	case "documentation":
		extractedContent, err = s.extractor.ExtractDocContent(html)
	case "raw":
		extractedContent = html
	default:
		extractedContent, err = s.extractor.ExtractMainContent(html)
	}

	if err != nil {
		return &types.FetchResponse{
			URL:      rawURL,
			Content:  extractedContent,
			Warnings: []string{fmt.Sprintf("extraction issue: %v", err)},
		}, nil
	}

	title := s.extractor.ExtractTitle(html)
	metadata, _ := s.extractor.ExtractMetadata(html)

	return &types.FetchResponse{
		URL:      rawURL,
		Title:    title,
		Content:  extractedContent,
		Metadata: metadata,
	}, nil
}

func (s *FetcherService) ExtractStructured(ctx context.Context, rawURL string) (*types.FetchResponse, error) {
	if err := s.isAllowedURL(rawURL); err != nil {
		return &types.FetchResponse{
			URL:      rawURL,
			Warnings: []string{err.Error()},
		}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; IA-Buscar/1.0; +https://thiscloud.es)")

	resp, err := s.client.Do(req)
	if err != nil {
		return &types.FetchResponse{
			URL:      rawURL,
			Warnings: []string{fmt.Sprintf("request failed: %v", err)},
		}, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 50*1024*1024))
	if err != nil {
		return &types.FetchResponse{
			URL:      rawURL,
			Warnings: []string{"failed to read response body"},
		}, fmt.Errorf("failed to read response: %w", err)
	}

	html := string(body)

	tables, _ := s.extractor.ExtractTables(html)
	metadata, _ := s.extractor.ExtractMetadata(html)

	structuredData := map[string]interface{}{
		"tables":   tables,
		"metadata": metadata,
	}

	return &types.FetchResponse{
		URL:      rawURL,
		Content:  fmt.Sprintf("%v", structuredData),
		Metadata: metadata,
	}, nil
}

func (s *FetcherService) ValidateURL(ctx context.Context, rawURL string) (bool, error) {
	if rawURL == "" {
		return false, fmt.Errorf("URL cannot be empty")
	}

	if err := s.isAllowedURL(rawURL); err != nil {
		return false, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodHead, rawURL, nil)
	if err != nil {
		return false, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; IA-Buscar/1.0; +https://thiscloud.es)")

	resp, err := s.client.Do(req)
	if err != nil {
		return false, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return false, fmt.Errorf("URL returned status %d", resp.StatusCode)
	}

	return true, nil
}

func (s *FetcherService) CheckLinkStatus(ctx context.Context, urls []string) ([]map[string]interface{}, error) {
	results := make([]map[string]interface{}, 0, len(urls))
	var wg sync.WaitGroup
	var mu sync.Mutex
	rateLimiter := time.NewTicker(200 * time.Millisecond)

	for _, rawURL := range urls {
		wg.Add(1)
		go func(u string) {
			defer wg.Done()
			<-rateLimiter.C

			result := map[string]interface{}{
				"url":    u,
				"valid":  false,
				"status": 0,
			}

			if err := s.isAllowedURL(u); err != nil {
				result["error"] = err.Error()
				mu.Lock()
				results = append(results, result)
				mu.Unlock()
				return
			}

			req, err := http.NewRequestWithContext(ctx, http.MethodHead, u, nil)
			if err != nil {
				result["error"] = err.Error()
				mu.Lock()
				results = append(results, result)
				mu.Unlock()
				return
			}

			req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; IA-Buscar/1.0; +https://thiscloud.es)")

			resp, err := s.client.Do(req)
			if err == nil {
				result["status"] = resp.StatusCode
				result["valid"] = resp.StatusCode < 400
				resp.Body.Close()
			} else {
				result["error"] = err.Error()
			}

			mu.Lock()
			results = append(results, result)
			mu.Unlock()
		}(rawURL)
	}

	wg.Wait()
	return results, nil
}
