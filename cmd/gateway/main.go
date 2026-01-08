package main

import (
	"context"
	"fmt"
	"main/internal/api/router"
	"main/internal/auth"
	"main/internal/config"
	"main/internal/loggers"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/joho/godotenv"
	"go.uber.org/zap"
)

func main() {
	// Load environment variables
	godotenv.Load()

	// Initialize logger
	log, err := loggers.NewLogger()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer log.Sync()

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatal("Failed to load configuration", zap.Error(err))
	}

	log.Info("Starting Fiber Gateway",
		zap.String("environment", cfg.Environment),
		zap.String("port", cfg.Server.Port),
		zap.String("nestjs_backend", "http://localhost:3000"),
	)

	// Create Fiber app
	app := fiber.New(fiber.Config{
		AppName: "Payment Gateway",
		Prefork: cfg.Environment == "production",
	})

	// Initialize JWT validator
	tokenValidator := auth.NewTokenValidator(cfg, log)

	// Setup all routes (core + optional features as needed)
	router.SetupRouter(app, cfg, log, tokenValidator)

	// Uncomment features as needed:
	// api.setupRateLimitingRoutes(app, cfg, log)
	// api.setupCircuitBreakerRoutes(app, cfg, log)
	// api.setupCachingRoutes(app, cfg, log)
	// api.setupMonitoringRoutes(app, cfg, log)
	// api.setupMetricsRoutes(app, cfg, log)

	// 404 handler for undefined routes
	app.Use(func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "route not found",
			"path":  c.Path(),
		})
	})

	// Start server in a goroutine
	addr := ":" + cfg.Server.Port
	go func() {
		log.Info("Server starting", zap.String("addr", addr))
		if err := app.Listen(addr); err != nil && err != fiber.ErrNotFound {
			log.Fatal("Server error", zap.Error(err))
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	<-quit
	log.Info("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := app.ShutdownWithContext(ctx); err != nil {
		log.Fatal("Server shutdown error", zap.Error(err))
	}

	log.Info("Server stopped gracefully")
}
