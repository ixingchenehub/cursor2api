package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"cursor2api/config"
	"cursor2api/handler"
	"cursor2api/logger"
	"cursor2api/middleware"
	"cursor2api/models"
	"cursor2api/service"
	"github.com/joho/godotenv"
)

func main() {
	// Load .env file at the very beginning
	if err := godotenv.Load(); err != nil {
		log.Printf("⚠️  Warning: .env file not found or cannot be loaded: %v", err)
		log.Println("ℹ️  Will use system environment variables or default values")
	} else {
		log.Println("✅ Successfully loaded .env file")
	}

	// Load configuration
	cfg := config.Load()
	
	// Set global config for other packages to access
	config.GlobalConfig = cfg

	// Initialize logger
	logger.Init(cfg.Logger.Level, cfg.Logger.Verbose)
	
	// Log startup information with emoji for better readability
	logger.Info("🚀 Starting cursor2api server")
	logger.Info("📋 Configuration loaded:")
	logger.Info("   ├─ Server Port: %s", cfg.Server.Port)
	logger.Info("   ├─ Log Level: %s", cfg.Logger.Level)
	logger.Info("   ├─ Auth Enabled: %v", cfg.Auth.Enabled)
	logger.Info("   ├─ Rate Limit Enabled: %v", cfg.RateLimit.Enabled)
	if cfg.RateLimit.Enabled {
		logger.Info("   ├─ Rate Limit: %.0f req/sec (burst: %d)", cfg.RateLimit.RequestsPerSec, cfg.RateLimit.Burst)
	}
	logger.Info("   └─ Process URL: %s", cfg.Cursor.ProcessURL)

	// Initialize AntiBot Manager
	antiBotManager := models.NewAntiBotManager(
		cfg.Cursor.JSURL,
		cfg.Cursor.ProcessURL,
		cfg.Cursor.RefreshInterval,
		cfg.Cursor.IdleTimeout,
	)

	// Start AntiBot Manager
	logger.Info("🔧 Initializing AntiBot Manager...")
	if err := antiBotManager.Start(); err != nil {
		logger.Error("❌ Failed to start AntiBot manager | error=%v", err)
		os.Exit(1)
	}
	defer antiBotManager.Stop()
	logger.Info("✅ AntiBot Manager started successfully")

	// Initialize Cursor Service
	cursorService := service.NewCursorService(antiBotManager, cfg.Cursor.SystemPrompt)

	// Initialize API Handler
	apiHandler := handler.NewAPIHandler(cursorService, antiBotManager, cfg)

	// Initialize API key authentication middleware
	authMiddleware := middleware.NewAPIKeyAuth(cfg.Auth.APIKeys, cfg.Auth.Enabled)

	// Initialize rate limiter middleware
	rateLimiter := middleware.NewRateLimiter(
		cfg.RateLimit.RequestsPerSec,
		cfg.RateLimit.Burst,
		cfg.RateLimit.Strategy,
		cfg.RateLimit.Enabled,
		cfg.RateLimit.CleanupInterval,
	)

	// Setup HTTP router
	mux := http.NewServeMux()

	// Health check endpoint (no authentication required)
	mux.HandleFunc("/health", apiHandler.HandleHealth)

	// OpenAI-compatible endpoints (authentication required)
	mux.HandleFunc("/v1/models", apiHandler.HandleModels)
	mux.HandleFunc("/v1/chat/completions", apiHandler.HandleChatCompletions)

	// Apply middleware chain: CORS -> RateLimit -> Auth -> Router
	handlerChain := middleware.CORS(rateLimiter.Middleware(authMiddleware.Middleware(mux)))

	// Create HTTP server
	server := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.Server.Port),
		Handler:      handlerChain,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 120 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in a goroutine for graceful shutdown
	go func() {
		logger.Info("🌐 Server listening on %s", server.Addr)
		logger.Info("📡 API Endpoints:")
		logger.Info("   ├─ GET  /health")
		logger.Info("   ├─ GET  /v1/models")
		logger.Info("   └─ POST /v1/chat/completions")
		logger.Info("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		logger.Info("✨ Server is ready to accept requests!")
		
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("❌ Server failed | error=%v", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("🛑 Shutdown signal received, gracefully shutting down...")

	// Create a deadline for shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Attempt graceful shutdown
	if err := server.Shutdown(ctx); err != nil {
		logger.Error("⚠️  Server forced to shutdown: %v", err)
	}

	logger.Info("👋 Server exited gracefully")
}