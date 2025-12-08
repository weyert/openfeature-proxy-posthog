package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/openfeature/posthog-proxy/internal/config"
	"github.com/openfeature/posthog-proxy/internal/handlers"
	"github.com/openfeature/posthog-proxy/internal/posthog"
	"github.com/openfeature/posthog-proxy/internal/telemetry"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	// Load environment variables (prioritize .env.local over .env)
	if err := godotenv.Load(".env.local"); err != nil {
		if err := godotenv.Load(); err != nil {
			slog.Info("No .env file found, using system environment variables")
		}
	}

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		slog.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}

	// Initialize Telemetry
	ctx := context.Background()
	shutdown, err := telemetry.InitProvider(ctx, cfg.Telemetry)
	if err != nil {
		slog.Error("Failed to initialize telemetry", "error", err)
		// Ensure shutdown is nil if init failed, though it likely is
		shutdown = nil
	}

	// Setup Logger (Hybrid: OTLP + Stdout)
	telemetry.SetupLogger(telemetry.GetLoggerProvider(), cfg.Telemetry.ServiceName)

	// Initialize Metrics
	metrics, err := telemetry.NewMetrics()
	if err != nil {
		slog.Error("Failed to initialize metrics", "error", err)
		os.Exit(1)
	}

	// Initialize PostHog client with insecure mode flag for logging
	posthogClient := posthog.NewClient(cfg.PostHog, cfg.Proxy.InsecureMode)

	// Initialize handlers
	handler := handlers.NewHandler(posthogClient, cfg, metrics)

	// Setup router
	router := gin.Default()

	// Add OpenTelemetry Middleware
	router.Use(otelgin.Middleware(cfg.Telemetry.ServiceName))

	// Prometheus Metrics Endpoint
	if cfg.Telemetry.Prometheus {
		router.GET("/metrics", gin.WrapH(promhttp.Handler()))
	}

	// Health check (always unauthenticated)
	router.GET("/health", func(c *gin.Context) {
		status := gin.H{
			"status":  "healthy",
			"version": version,
			"commit":  commit,
			"date":    date,
		}
		
		// Add insecure mode warning if enabled
		if cfg.Proxy.InsecureMode {
			status["warning"] = "Running in INSECURE MODE - authentication disabled"
		}
		
		c.JSON(200, status)
	})

	// OpenFeature API routes
	api := router.Group("/openfeature/v0")
	
	// Apply authentication middleware
	api.Use(handler.AuthMiddleware())
	
	{
		// Read operations (require 'read' capability)
		api.GET("/manifest", handler.RequireCapability("read"), handler.GetManifest)
		
		// Write operations (require 'write' capability)
		api.POST("/manifest/flags", handler.RequireCapability("write"), handler.CreateFlag)
		api.PUT("/manifest/flags/:key", handler.RequireCapability("write"), handler.UpdateFlag)
		
		// Delete operations (require 'delete' capability)
		api.DELETE("/manifest/flags/:key", handler.RequireCapability("delete"), handler.DeleteFlag)
	}

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: router,
	}

	// Display startup information
	slog.Info("Starting PostHog OpenFeature proxy", "port", port)
	if cfg.Proxy.InsecureMode {
		slog.Warn("Running in INSECURE MODE - Authentication is disabled!")
		slog.Warn("This should ONLY be used for development and testing!")
		slog.Info("API request/response logging enabled - check ./logs/ directory")
	} else {
		slog.Info("Authentication enabled", "token_count", len(cfg.Proxy.Auth.Tokens))
	}

	// Initializing the server in a goroutine so that
	// it won't block the graceful shutdown handling below
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Failed to start server", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server with
	// a timeout of 5 seconds.
	quit := make(chan os.Signal, 1)
	// kill (no param) default send syscall.SIGTERM
	// kill -2 is syscall.SIGINT
	// kill -9 is syscall.SIGKILL but can't be caught, so don't need to add it
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	slog.Info("Shutting down server...")

	// The context is used to inform the server it has 5 seconds to finish
	// the request it is currently handling
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("Server forced to shutdown", "error", err)
	}

	// Shutdown telemetry
	if shutdown != nil {
		if err := shutdown(context.Background()); err != nil {
			slog.Error("Failed to shutdown telemetry", "error", err)
		}
	}

	slog.Info("Server exiting")
}