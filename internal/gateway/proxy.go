package gateway

import (
	"context"
	"fmt"
	"io"
	"main/internal/config"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/sony/gobreaker"
	"go.uber.org/zap"
)

type Proxy struct {
	config          *config.Config
	logger          *zap.Logger
	client          *http.Client
	circuitBreakers map[string]*gobreaker.CircuitBreaker
	services        map[string]*config.ServiceConfig
}

type ProxyRequest struct {
	OriginalRequest *http.Request
	TargetURL       *url.URL
	ServiceName     string
}

type ProxyResponse struct {
	StatusCode int
	Headers    http.Header
	Body       []byte
}

func NewProxy(cfg *config.Config, log *zap.Logger) *Proxy {
	p := &Proxy{
		config:          cfg,
		logger:          log,
		circuitBreakers: make(map[string]*gobreaker.CircuitBreaker),
		services:        make(map[string]*config.ServiceConfig),
		client: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				MaxConnsPerHost:     10,
			},
		},
	}

	// Initialize circuit breakers and services map
	for _, service := range cfg.Upstream.Services {
		svc := service // Copy for pointer
		p.services[service.Name] = &svc

		settings := gobreaker.Settings{
			Name:        service.Name,
			MaxRequests: 10,
			Interval:    time.Second,
			Timeout:     5 * time.Second,
			ReadyToTrip: func(counts gobreaker.Counts) bool {
				failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
				return counts.Requests >= 3 && failureRatio >= 0.6
			},
		}

		p.circuitBreakers[service.Name] = gobreaker.NewCircuitBreaker(settings)
	}

	p.logger.Info("Proxy initialized with services",
		zap.Int("count", len(cfg.Upstream.Services)),
	)

	return p
}

// RouteRequest routes request to appropriate upstream service
func (p *Proxy) RouteRequest(req *http.Request, serviceName string) (*ProxyResponse, error) {
	service, exists := p.services[serviceName]
	if !exists {
		return nil, fmt.Errorf("service not found: %s", serviceName)
	}

	cb := p.circuitBreakers[serviceName]

	// Execute with circuit breaker
	result, err := cb.Execute(func() (interface{}, error) {
		return p.executeRequest(req, service)
	})

	if err != nil {
		p.logger.Error("Request execution failed",
			zap.String("service", serviceName),
			zap.Error(err),
		)
		return nil, err
	}

	return result.(*ProxyResponse), nil
}

func (p *Proxy) executeRequest(req *http.Request, service *config.ServiceConfig) (*ProxyResponse, error) {
	// Build target URL
	targetURL, err := url.Parse(service.URL)
	if err != nil {
		return nil, fmt.Errorf("invalid service URL: %w", err)
	}

	// Preserve path and query from original request
	targetURL.Path = req.URL.Path
	targetURL.RawQuery = req.URL.RawQuery

	// Create new request
	proxyReq, err := http.NewRequest(req.Method, targetURL.String(), req.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to create proxy request: %w", err)
	}

	// Copy headers from original request
	p.copyHeaders(req.Header, proxyReq.Header)

	// Set request context and timeout
	ctx, cancel := context.WithTimeout(req.Context(),
		time.Duration(service.Timeout)*time.Second)
	defer cancel()
	proxyReq = proxyReq.WithContext(ctx)

	// Execute request with retry logic
	var resp *http.Response
	for attempt := 0; attempt < service.MaxRetry; attempt++ {
		resp, err = p.client.Do(proxyReq)
		if err == nil {
			break
		}

		p.logger.Warn("Request attempt failed",
			zap.String("service", service.Name),
			zap.Int("attempt", attempt+1),
			zap.Error(err),
		)

		if attempt < service.MaxRetry-1 {
			time.Sleep(time.Duration((attempt+1)*100) * time.Millisecond)
		}
	}

	if err != nil {
		return nil, fmt.Errorf("all retry attempts failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	p.logger.Debug("Request routed successfully",
		zap.String("service", service.Name),
		zap.Int("status_code", resp.StatusCode),
		zap.Int("response_size", len(body)),
	)

	return &ProxyResponse{
		StatusCode: resp.StatusCode,
		Headers:    resp.Header,
		Body:       body,
	}, nil
}

func (p *Proxy) copyHeaders(src http.Header, dst http.Header) {
	// Headers to skip
	skipHeaders := map[string]bool{
		"host":              true,
		"connection":        true,
		"content-length":    true,
		"transfer-encoding": true,
		"upgrade":           true,
	}

	for key, values := range src {
		if skipHeaders[strings.ToLower(key)] {
			continue
		}

		for _, value := range values {
			dst.Add(key, value)
		}
	}
}

// GetServiceHealth returns health status of a service
func (p *Proxy) GetServiceHealth(serviceName string) string {
	cb, exists := p.circuitBreakers[serviceName]
	if !exists {
		return "unknown"
	}

	state := cb.State()
	return state.String()
}

// GetAllServiceStatus returns health status of all services
func (p *Proxy) GetAllServiceStatus() map[string]string {
	status := make(map[string]string)
	for serviceName := range p.circuitBreakers {
		status[serviceName] = p.GetServiceHealth(serviceName)
	}
	return status
}
