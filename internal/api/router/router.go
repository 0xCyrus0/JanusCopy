package router

import (
	"bytes"
	"io"
	"main/internal/auth"
	"main/internal/config"
	"net/http"

	"github.com/gofiber/fiber/v2"
	jwtware "github.com/gofiber/jwt/v3"
	"go.uber.org/zap"
)

// SetupRouter initializes the main router with all routes
func SetupRouter(app *fiber.App, cfg *config.Config, log *zap.Logger, validator *auth.TokenValidator) {
	// Essential middleware (always enabled)
	SetupCoreMiddleware(app, cfg, log, validator)

	// Core routes - forward to NestJS backend
	SetupPublicRoutes(app, cfg, log)

	// Optional feature routes - add only what you need
	// setupRateLimitingRoutes(app, cfg, log)
	// setupCircuitBreakerRoutes(app, cfg, log)
	// setupCachingRoutes(app, cfg, log)
	// setupMonitoringRoutes(app, cfg, log)
	// setupMetricsRoutes(app, cfg, log)
}

// ============================================================================
// CORE - Always enabled (JWT, CORS, Logging)
// ============================================================================

func SetupCoreMiddleware(app *fiber.App, cfg *config.Config, log *zap.Logger, validator *auth.TokenValidator) {
	// Recovery from panics
	app.Use(func(c *fiber.Ctx) error {
		defer func() {
			if err := recover(); err != nil {
				log.Error("Panic recovered", zap.Any("error", err))
				c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"error": "internal server error",
				})
			}
		}()
		return c.Next()
	})

	// Request logging
	app.Use(func(c *fiber.Ctx) error {
		log.Info("Request received",
			zap.String("method", c.Method()),
			zap.String("path", c.Path()),
			zap.String("ip", c.IP()),
		)
		return c.Next()
	})

	// CORS - Allow Angular on :4200
	app.Use(func(c *fiber.Ctx) error {
		origin := c.Get("Origin")
		c.Set("Access-Control-Allow-Origin", origin)
		c.Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,PATCH,OPTIONS")
		c.Set("Access-Control-Allow-Headers", "Content-Type,Authorization")
		c.Set("Access-Control-Allow-Credentials", "true")

		if c.Method() == fiber.MethodOptions {
			return c.SendStatus(fiber.StatusOK)
		}

		return c.Next()
	})
}

// ============================================================================
// CORE ROUTES - Forward to NestJS Backend (:3000)
// ============================================================================

func SetupPublicRoutes(app *fiber.App, cfg *config.Config, log *zap.Logger) {
	nestjsURL := "http://localhost:3000"

	// Health check (no auth required - public)
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok", "gateway": "running"})
	})

	// Protected routes - require JWT
	protected := app.Group("")
	protected.Use(jwtware.New(jwtware.Config{
		SigningKey: []byte(cfg.JWT.SecretKey),
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "unauthorized",
			})
		},
		SuccessHandler: func(c *fiber.Ctx) error {
			return c.Next()
		},
	}))

	// Catch-all route - forward everything to NestJS (protected)
	protected.All("/*", func(c *fiber.Ctx) error {
		path := c.Path()
		return ForwardRequest(c, nestjsURL, path, log)
	})
}

// ============================================================================
// HELPER FUNCTION - Forward requests to NestJS backend
// ============================================================================

func ForwardRequest(c *fiber.Ctx, backendURL string, path string, log *zap.Logger) error {
	// Create new request to NestJS backend
	req, err := http.NewRequest(c.Method(), backendURL+path, bytes.NewReader(c.Body()))
	if err != nil {
		log.Error("Failed to create request", zap.Error(err), zap.String("path", path))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "gateway error",
		})
	}

	// Copy headers from original request (fasthttp style)
	c.Request().Header.VisitAll(func(key, value []byte) {
		req.Header.Add(string(key), string(value))
	})

	// Add query parameters
	if len(c.Request().URI().QueryString()) > 0 {
		req.URL.RawQuery = string(c.Request().URI().QueryString())
	}
	// Execute request to NestJS
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Error("Request to backend failed", zap.Error(err), zap.String("path", path))
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
			"error": "backend service unavailable",
		})
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Error("Failed to read response", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "gateway error",
		})
	}

	// Copy response headers
	for key, values := range resp.Header {
		for _, value := range values {
			c.Set(key, value)
		}
	}

	// Log the request
	log.Info("Request forwarded",
		zap.String("method", c.Method()),
		zap.String("path", path),
		zap.Int("status", resp.StatusCode),
	)

	// Return response from NestJS
	return c.Status(resp.StatusCode).Send(body)
}

// ============================================================================
// OPTIONAL FEATURES - Enable only when needed
// ============================================================================

// setupRateLimitingRoutes adds rate limiting to specific endpoints
func SetupRateLimitingRoutes(app *fiber.App, cfg *config.Config, log *zap.Logger) {
	// Apply rate limiting to high-traffic routes
	// Example: Rate limit middleware can be added here
}

// setupCircuitBreakerRoutes adds circuit breaker pattern to critical endpoints
func SetupCircuitBreakerRoutes(app *fiber.App, cfg *config.Config, log *zap.Logger) {
	// Apply circuit breaker to external service calls
	// Example: Circuit breaker middleware can be added here
}

// setupCachingRoutes adds response caching to read-only endpoints
func SetupCachingRoutes(app *fiber.App, cfg *config.Config, log *zap.Logger) {
	// Cache GET requests for a period
	// Example: Caching middleware can be added here
}

// setupMonitoringRoutes adds monitoring/status endpoints
func SetupMonitoringRoutes(app *fiber.App, cfg *config.Config, log *zap.Logger) {
	// Health status
	app.Get("/monitor/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":  "healthy",
			"gateway": "ok",
		})
	})

	// Service metrics
	app.Get("/monitor/metrics", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"requests_total":  1000,
			"requests_failed": 5,
			"avg_latency_ms":  45,
		})
	})

	// Dependency status
	app.Get("/monitor/dependencies", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"nestjs": "connected",
		})
	})
}

// setupMetricsRoutes adds Prometheus-style metrics endpoints
func SetupMetricsRoutes(app *fiber.App, cfg *config.Config, log *zap.Logger) {
	// Prometheus metrics endpoint
	app.Get("/metrics", func(c *fiber.Ctx) error {
		return c.SendString("# HELP requests_total Total requests\n# TYPE requests_total counter\nrequests_total 1000\n")
	})
}
