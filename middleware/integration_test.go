package middleware

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"cursor2api/config"
)

// TestRateLimitIntegration_FullMiddlewareChain tests the complete middleware chain
// including CORS, RateLimit, and Auth in the correct order
func TestRateLimitIntegration_FullMiddlewareChain(t *testing.T) {
	// Setup
	gin.SetMode(gin.TestMode)
	
	cfg := &config.Config{
		RateLimit: config.RateLimitConfig{
			Enabled:         true,
			RequestsPerSec:  2.0,
			Burst:           2,
			Strategy:        "ip",
			CleanupInterval: 1 * time.Minute,
		},
		APIKeys: []string{"test-key-123"},
	}
	
	router := gin.New()
	
	// Apply middleware in production order: CORS -> RateLimit -> Auth
	router.Use(CORS())
	router.Use(NewRateLimiter(cfg).Middleware())
	router.Use(NewAPIKeyAuth(cfg).Middleware())
	
	// Test endpoint
	router.POST("/v1/chat/completions", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "success"})
	})
	
	// Test 1: First request should pass all middleware
	t.Run("FirstRequestPassesAllMiddleware", func(t *testing.T) {
		req := createTestRequest(t, "test-key-123")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		
		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
		}
		
		// Verify CORS headers are present
		if w.Header().Get("Access-Control-Allow-Origin") == "" {
			t.Error("CORS headers missing")
		}
	})
	
	// Test 2: Rate limit should trigger before auth
	t.Run("RateLimitTriggersBeforeAuth", func(t *testing.T) {
		// Exhaust rate limit with valid key
		for i := 0; i < 2; i++ {
			req := createTestRequest(t, "test-key-123")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
		}
		
		// Next request should be rate limited (even with valid key)
		req := createTestRequest(t, "test-key-123")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		
		if w.Code != http.StatusTooManyRequests {
			t.Errorf("Expected status 429, got %d", w.Code)
		}
		
		// Verify rate limit headers
		if w.Header().Get("X-RateLimit-Limit") == "" {
			t.Error("X-RateLimit-Limit header missing")
		}
		if w.Header().Get("Retry-After") == "" {
			t.Error("Retry-After header missing")
		}
	})
	
	// Test 3: Invalid API key should fail auth (after passing rate limit)
	t.Run("InvalidKeyFailsAuth", func(t *testing.T) {
		// Wait for rate limit to reset
		time.Sleep(1 * time.Second)
		
		req := createTestRequest(t, "invalid-key")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		
		if w.Code != http.StatusUnauthorized {
			t.Errorf("Expected status 401, got %d", w.Code)
		}
	})
}

// TestRateLimitIntegration_HealthCheckExemption verifies that health check
// endpoints bypass rate limiting
func TestRateLimitIntegration_HealthCheckExemption(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	cfg := &config.Config{
		RateLimit: config.RateLimitConfig{
			Enabled:         true,
			RequestsPerSec:  1.0,
			Burst:           1,
			Strategy:        "ip",
			CleanupInterval: 1 * time.Minute,
		},
	}
	
	router := gin.New()
	router.Use(NewRateLimiter(cfg).Middleware())
	
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})
	
	// Make multiple health check requests rapidly
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		
		if w.Code != http.StatusOK {
			t.Errorf("Health check request %d failed with status %d", i+1, w.Code)
		}
	}
}

// TestRateLimitIntegration_ErrorResponseFormat verifies that rate limit
// error responses conform to OpenAI API format
func TestRateLimitIntegration_ErrorResponseFormat(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	cfg := &config.Config{
		RateLimit: config.RateLimitConfig{
			Enabled:         true,
			RequestsPerSec:  1.0,
			Burst:           1,
			Strategy:        "ip",
			CleanupInterval: 1 * time.Minute,
		},
	}
	
	router := gin.New()
	router.Use(NewRateLimiter(cfg).Middleware())
	
	router.POST("/v1/chat/completions", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "success"})
	})
	
	// Exhaust rate limit
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	}
	
	// Trigger rate limit
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	
	// Verify response format
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("Expected status 429, got %d", w.Code)
	}
	
	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	
	// Verify OpenAI-compatible error structure
	errorObj, ok := response["error"].(map[string]interface{})
	if !ok {
		t.Fatal("Response missing 'error' object")
	}
	
	if errorObj["message"] == nil {
		t.Error("Error object missing 'message' field")
	}
	
	if errorObj["type"] == nil {
		t.Error("Error object missing 'type' field")
	}
	
	if errorObj["code"] == nil {
		t.Error("Error object missing 'code' field")
	}
	
	// Verify required headers
	requiredHeaders := []string{
		"X-RateLimit-Limit",
		"X-RateLimit-Remaining",
		"X-RateLimit-Reset",
		"Retry-After",
	}
	
	for _, header := range requiredHeaders {
		if w.Header().Get(header) == "" {
			t.Errorf("Missing required header: %s", header)
		}
	}
}

// TestRateLimitIntegration_IPStrategy tests IP-based rate limiting
// with different client IPs
func TestRateLimitIntegration_IPStrategy(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	cfg := &config.Config{
		RateLimit: config.RateLimitConfig{
			Enabled:         true,
			RequestsPerSec:  1.0,
			Burst:           1,
			Strategy:        "ip",
			CleanupInterval: 1 * time.Minute,
		},
	}
	
	router := gin.New()
	router.Use(NewRateLimiter(cfg).Middleware())
	
	router.POST("/v1/chat/completions", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "success"})
	})
	
	// Client 1 exhausts its limit
	req1 := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	req1.RemoteAddr = "192.168.1.1:12345"
	
	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req1)
	}
	
	// Client 1 should be rate limited
	w1 := httptest.NewRecorder()
	router.ServeHTTP(w1, req1)
	if w1.Code != http.StatusTooManyRequests {
		t.Errorf("Client 1 should be rate limited, got status %d", w1.Code)
	}
	
	// Client 2 should still be able to make requests
	req2 := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	req2.RemoteAddr = "192.168.1.2:12345"
	
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Errorf("Client 2 should not be rate limited, got status %d", w2.Code)
	}
}

// TestRateLimitIntegration_APIKeyStrategy tests API key-based rate limiting
func TestRateLimitIntegration_APIKeyStrategy(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	cfg := &config.Config{
		RateLimit: config.RateLimitConfig{
			Enabled:         true,
			RequestsPerSec:  1.0,
			Burst:           1,
			Strategy:        "api_key",
			CleanupInterval: 1 * time.Minute,
		},
		APIKeys: []string{"key1", "key2"},
	}
	
	router := gin.New()
	router.Use(NewRateLimiter(cfg).Middleware())
	router.Use(NewAPIKeyAuth(cfg).Middleware())
	
	router.POST("/v1/chat/completions", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "success"})
	})
	
	// Key1 exhausts its limit
	for i := 0; i < 2; i++ {
		req := createTestRequest(t, "key1")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	}
	
	// Key1 should be rate limited
	req1 := createTestRequest(t, "key1")
	w1 := httptest.NewRecorder()
	router.ServeHTTP(w1, req1)
	if w1.Code != http.StatusTooManyRequests {
		t.Errorf("Key1 should be rate limited, got status %d", w1.Code)
	}
	
	// Key2 should still work
	req2 := createTestRequest(t, "key2")
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Errorf("Key2 should not be rate limited, got status %d", w2.Code)
	}
}

// TestRateLimitIntegration_XForwardedFor tests X-Forwarded-For header handling
func TestRateLimitIntegration_XForwardedFor(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	cfg := &config.Config{
		RateLimit: config.RateLimitConfig{
			Enabled:         true,
			RequestsPerSec:  1.0,
			Burst:           1,
			Strategy:        "ip",
			CleanupInterval: 1 * time.Minute,
		},
	}
	
	router := gin.New()
	router.Use(NewRateLimiter(cfg).Middleware())
	
	router.POST("/v1/chat/completions", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "success"})
	})
	
	// Exhaust limit for IP from X-Forwarded-For
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
		req.Header.Set("X-Forwarded-For", "203.0.113.1")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	}
	
	// Same IP should be rate limited
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.1")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("Expected rate limit for X-Forwarded-For IP, got status %d", w.Code)
	}
}

// TestRateLimitIntegration_DisabledMode verifies that when rate limiting
// is disabled, all requests pass through
func TestRateLimitIntegration_DisabledMode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	cfg := &config.Config{
		RateLimit: config.RateLimitConfig{
			Enabled:         false,
			RequestsPerSec:  1.0,
			Burst:           1,
			Strategy:        "ip",
			CleanupInterval: 1 * time.Minute,
		},
	}
	
	router := gin.New()
	router.Use(NewRateLimiter(cfg).Middleware())
	
	router.POST("/v1/chat/completions", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "success"})
	})
	
	// Make many requests rapidly
	for i := 0; i < 100; i++ {
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		
		if w.Code != http.StatusOK {
			t.Errorf("Request %d failed with status %d (rate limiting should be disabled)", i+1, w.Code)
		}
	}
}

// Helper function to create a test request with API key
func createTestRequest(t *testing.T, apiKey string) *http.Request {
	t.Helper()
	
	body := map[string]interface{}{
		"model": "gpt-4",
		"messages": []map[string]string{
			{"role": "user", "content": "test"},
		},
	}
	
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("Failed to marshal request body: %v", err)
	}
	
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	
	return req
}