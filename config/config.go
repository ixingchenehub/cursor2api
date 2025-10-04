package config

import (
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all application configuration
type Config struct {
	Server    ServerConfig
	Logger    LoggerConfig
	Cursor    CursorConfig
	Auth      AuthConfig
	RateLimit RateLimitConfig
}

// ServerConfig holds server-related configuration
type ServerConfig struct {
	Port string
}

// LoggerConfig holds logger-related configuration
type LoggerConfig struct {
	Level   string
	Verbose bool
}

// CursorConfig holds cursor-specific configuration
type CursorConfig struct {
	JSURL                  string
	ProcessURL             string
	SystemPrompt           string
	RefreshInterval        time.Duration
	IdleTimeout            time.Duration
	EnableFunctionCalling  bool
}

// AuthConfig holds authentication-related configuration
type AuthConfig struct {
	Enabled bool
	APIKeys []string
}

// RateLimitConfig holds rate limiting configuration
type RateLimitConfig struct {
	Enabled         bool
	RequestsPerSec  float64
	Burst           int
	Strategy        string
	CleanupInterval time.Duration
}

// Load reads configuration from environment variables
func Load() *Config {
	cfg := &Config{
		Server: ServerConfig{
			Port: getEnv("PORT", "5680"),
		},
		Logger: LoggerConfig{
			Level:   getEnv("LOG_LEVEL", "info"),
			Verbose: getBoolEnv("LOG_VERBOSE", false),
		},
		Cursor: CursorConfig{
			JSURL:           getEnv("JS_URL", "https://cursor.com/149e9513-01fa-4fb0-aad4-566afd725d1b/2d206a39-8ed7-437e-a3be-862e0f06eea3/a-4-a/c.js?i=0&v=3&h=cursor.com"),
			ProcessURL:      getEnv("PROCESS_URL", "http://localhost:3000/api/process"),
			SystemPrompt:    getEnv("SYSTEM_PROMPT", "You are a helpful assistant."),
			RefreshInterval: getDurationEnv("REFRESH_INTERVAL", 5*time.Minute),
			IdleTimeout:     getDurationEnv("IDLE_TIMEOUT", 10*time.Minute),
		},
		Auth: AuthConfig{
			Enabled: getBoolEnv("AUTH_ENABLED", true),
			APIKeys: getSliceEnv("API_KEYS", []string{}),
		},
		RateLimit: RateLimitConfig{
			Enabled:         getBoolEnv("RATE_LIMIT_ENABLED", true),
			RequestsPerSec:  getFloatEnv("RATE_LIMIT_REQUESTS_PER_SEC", 1000.0),
			Burst:           getIntEnv("RATE_LIMIT_BURST", 2000),
			Strategy:        getEnv("RATE_LIMIT_STRATEGY", "ip"),
			CleanupInterval: getDurationEnv("RATE_LIMIT_CLEANUP_INTERVAL", 10*time.Minute),
		},
	}

	// Validate required configuration
	if cfg.Cursor.ProcessURL == "" || cfg.Cursor.ProcessURL == "http://localhost:3000/api/process" {
		log.Println("⚠️  Warning: PROCESS_URL not configured or using default value")
		log.Println("   Please set PROCESS_URL in .env file to your actual AntiBot service endpoint")
	}

	if cfg.Auth.Enabled && len(cfg.Auth.APIKeys) == 0 {
		log.Println("⚠️  Warning: AUTH_ENABLED is true but no API_KEYS configured")
		log.Println("   Please set API_KEYS in .env file or disable authentication")
	}

	// Log loaded configuration with detailed information
	log.Println("✅ Configuration loaded successfully:")
	log.Printf("   ├─ Server Port: %s", cfg.Server.Port)
	log.Printf("   ├─ Log Level: %s (verbose: %v)", cfg.Logger.Level, cfg.Logger.Verbose)
	log.Printf("   ├─ Auth Enabled: %v", cfg.Auth.Enabled)
	if cfg.Auth.Enabled {
		log.Printf("   ├─ API Keys Count: %d", len(cfg.Auth.APIKeys))
	}
	log.Printf("   ├─ Rate Limit Enabled: %v", cfg.RateLimit.Enabled)
	if cfg.RateLimit.Enabled {
		log.Printf("   ├─ Rate Limit: %.0f req/sec (burst: %d, strategy: %s)", 
			cfg.RateLimit.RequestsPerSec, cfg.RateLimit.Burst, cfg.RateLimit.Strategy)
	}
	log.Printf("   ├─ Process URL: %s", cfg.Cursor.ProcessURL)
	log.Printf("   ├─ JS URL: %s", cfg.Cursor.JSURL)
	log.Printf("   ├─ Refresh Interval: %s", cfg.Cursor.RefreshInterval)
	log.Printf("   └─ Idle Timeout: %s", cfg.Cursor.IdleTimeout)

	return cfg
}

// getEnv retrieves a string environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getBoolEnv retrieves a boolean environment variable or returns a default value
func getBoolEnv(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		boolValue, err := strconv.ParseBool(value)
		if err != nil {
			log.Printf("⚠️  Warning: Invalid boolean value for %s: %s, using default: %v", key, value, defaultValue)
			return defaultValue
		}
		return boolValue
	}
	return defaultValue
}

// getIntEnv retrieves an integer environment variable or returns a default value
func getIntEnv(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		intValue, err := strconv.Atoi(value)
		if err != nil {
			log.Printf("⚠️  Warning: Invalid integer value for %s: %s, using default: %d", key, value, defaultValue)
			return defaultValue
		}
		return intValue
	}
	return defaultValue
}

// getFloatEnv retrieves a float environment variable or returns a default value
func getFloatEnv(key string, defaultValue float64) float64 {
	if value := os.Getenv(key); value != "" {
		floatValue, err := strconv.ParseFloat(value, 64)
		if err != nil {
			log.Printf("⚠️  Warning: Invalid float value for %s: %s, using default: %.2f", key, value, defaultValue)
			return defaultValue
		}
		return floatValue
	}
	return defaultValue
}

// getDurationEnv retrieves a duration environment variable or returns a default value
func getDurationEnv(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		duration, err := time.ParseDuration(value)
		if err != nil {
			log.Printf("⚠️  Warning: Invalid duration value for %s: %s, using default: %s", key, value, defaultValue)
			return defaultValue
		}
		return duration
	}
	return defaultValue
}

// getSliceEnv retrieves a comma-separated string environment variable as a slice
func getSliceEnv(key string, defaultValue []string) []string {
	if value := os.Getenv(key); value != "" {
		items := strings.Split(value, ",")
		result := make([]string, 0, len(items))
		for _, item := range items {
			if trimmed := strings.TrimSpace(item); trimmed != "" {
				result = append(result, trimmed)
			}
		}
		return result
	}
	return defaultValue
}

// GlobalConfig is the global configuration instance
var GlobalConfig *Config