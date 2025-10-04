package middleware

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// Test helper: create a simple next handler
func createTestHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}
}

// Test 1: RateLimiter creation and configuration
func TestNewRateLimiter(t *testing.T) {
	tests := []struct {
		name            string
		requestsPerSec  float64
		burst           int
		strategy        string
		enabled         bool
		cleanupInterval time.Duration
		wantPanic       bool
	}{
		{
			name:            "Valid IP strategy",
			requestsPerSec:  10.0,
			burst:           20,
			strategy:        "ip",
			enabled:         true,
			cleanupInterval: time.Hour,
			wantPanic:       false,
		},
		{
			name:            "Valid API Key strategy",
			requestsPerSec:  5.0,
			burst:           10,
			strategy:        "api_key",
			enabled:         true,
			cleanupInterval: 30 * time.Minute,
			wantPanic:       false,
		},
		{
			name:            "Disabled rate limiter",
			requestsPerSec:  10.0,
			burst:           20,
			strategy:        "ip",
			enabled:         false,
			cleanupInterval: time.Hour,
			wantPanic:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					if !tt.wantPanic {
						t.Errorf("NewRateLimiter() panicked unexpectedly: %v", r)
					}
				}
			}()

			rl := NewRateLimiter(
				tt.requestsPerSec,
				tt.burst,
				tt.strategy,
				tt.enabled,
				tt.cleanupInterval,
			)

			if rl == nil {
				t.Fatal("NewRateLimiter() returned nil")
			}

			if float64(rl.requestsPerSec) != tt.requestsPerSec {
				t.Errorf("requestsPerSec = %v, want %v", float64(rl.requestsPerSec), tt.requestsPerSec)
			}

			if rl.burst != tt.burst {
				t.Errorf("burst = %v, want %v", rl.burst, tt.burst)
			}

			if rl.strategy != tt.strategy {
				t.Errorf("strategy = %v, want %v", rl.strategy, tt.strategy)
			}

			if rl.enabled != tt.enabled {
				t.Errorf("enabled = %v, want %v", rl.enabled, tt.enabled)
			}

			if rl.cleanupInterval != tt.cleanupInterval {
				t.Errorf("cleanupInterval = %v, want %v", rl.cleanupInterval, tt.cleanupInterval)
			}
		})
	}
}

// Test 2: Normal requests pass through
func TestRateLimiter_AllowNormalRequests(t *testing.T) {
	rl := NewRateLimiter(10.0, 20, "ip", true, time.Hour)
	handler := rl.Middleware(createTestHandler())

	// Send requests within limit
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest("GET", "/api/test", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Request %d: got status %d, want %d", i+1, w.Code, http.StatusOK)
		}
	}
}

// Test 3: Requests exceeding limit are rejected
func TestRateLimiter_RejectExcessRequests(t *testing.T) {
	// Create a very restrictive rate limiter: 1 req/sec, burst 2
	rl := NewRateLimiter(1.0, 2, "ip", true, time.Hour)
	handler := rl.Middleware(createTestHandler())

	req := httptest.NewRequest("GET", "/api/test", nil)
	req.RemoteAddr = "192.168.1.1:12345"

	// First 2 requests should pass (burst)
	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("Request %d: got status %d, want %d", i+1, w.Code, http.StatusOK)
		}
	}

	// 3rd request should be rate limited
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("Excess request: got status %d, want %d", w.Code, http.StatusTooManyRequests)
	}

	// Verify response headers
	if w.Header().Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type = %v, want application/json", w.Header().Get("Content-Type"))
	}

	if w.Header().Get("Retry-After") == "" {
		t.Error("Retry-After header is missing")
	}

	if w.Header().Get("X-RateLimit-Limit") == "" {
		t.Error("X-RateLimit-Limit header is missing")
	}

	if w.Header().Get("X-RateLimit-Reset") == "" {
		t.Error("X-RateLimit-Reset header is missing")
	}
}

// Test 4: Health check endpoint is exempted
func TestRateLimiter_HealthCheckExemption(t *testing.T) {
	// Create a very restrictive rate limiter
	rl := NewRateLimiter(1.0, 1, "ip", true, time.Hour)
	handler := rl.Middleware(createTestHandler())

	req := httptest.NewRequest("GET", "/health", nil)
	req.RemoteAddr = "192.168.1.1:12345"

	// Send many requests to /health - all should pass
	for i := 0; i < 10; i++ {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("Health check request %d: got status %d, want %d", i+1, w.Code, http.StatusOK)
		}
	}
}

// Test 5: Disabled rate limiter allows all requests
func TestRateLimiter_DisabledAllowsAll(t *testing.T) {
	rl := NewRateLimiter(1.0, 1, "ip", false, time.Hour)
	handler := rl.Middleware(createTestHandler())

	req := httptest.NewRequest("GET", "/api/test", nil)
	req.RemoteAddr = "192.168.1.1:12345"

	// Send many requests - all should pass when disabled
	for i := 0; i < 100; i++ {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("Request %d: got status %d, want %d", i+1, w.Code, http.StatusOK)
		}
	}
}

// Test 6: IP strategy - different IPs have separate limits
func TestRateLimiter_IPStrategy_SeparateLimits(t *testing.T) {
	rl := NewRateLimiter(1.0, 2, "ip", true, time.Hour)
	handler := rl.Middleware(createTestHandler())

	// IP 1: exhaust its limit
	req1 := httptest.NewRequest("GET", "/api/test", nil)
	req1.RemoteAddr = "192.168.1.1:12345"

	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req1)
		if w.Code != http.StatusOK {
			t.Errorf("IP1 Request %d: got status %d, want %d", i+1, w.Code, http.StatusOK)
		}
	}

	// IP 1: next request should be rate limited
	w1 := httptest.NewRecorder()
	handler.ServeHTTP(w1, req1)
	if w1.Code != http.StatusTooManyRequests {
		t.Errorf("IP1 excess request: got status %d, want %d", w1.Code, http.StatusTooManyRequests)
	}

	// IP 2: should still have its own limit
	req2 := httptest.NewRequest("GET", "/api/test", nil)
	req2.RemoteAddr = "192.168.1.2:12345"

	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Errorf("IP2 request: got status %d, want %d", w2.Code, http.StatusOK)
	}
}

// Test 7: API Key strategy - different keys have separate limits
func TestRateLimiter_APIKeyStrategy_SeparateLimits(t *testing.T) {
	rl := NewRateLimiter(1.0, 2, "api_key", true, time.Hour)
	handler := rl.Middleware(createTestHandler())

	// API Key 1: exhaust its limit
	req1 := httptest.NewRequest("GET", "/api/test", nil)
	req1.Header.Set("Authorization", "Bearer key_12345678")
	req1.RemoteAddr = "192.168.1.1:12345"

	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req1)
		if w.Code != http.StatusOK {
			t.Errorf("Key1 Request %d: got status %d, want %d", i+1, w.Code, http.StatusOK)
		}
	}

	// API Key 1: next request should be rate limited
	w1 := httptest.NewRecorder()
	handler.ServeHTTP(w1, req1)
	if w1.Code != http.StatusTooManyRequests {
		t.Errorf("Key1 excess request: got status %d, want %d", w1.Code, http.StatusTooManyRequests)
	}

	// API Key 2: should still have its own limit
	req2 := httptest.NewRequest("GET", "/api/test", nil)
	req2.Header.Set("Authorization", "Bearer key_87654321")
	req2.RemoteAddr = "192.168.1.1:12345"

	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Errorf("Key2 request: got status %d, want %d", w2.Code, http.StatusOK)
	}
}

// Test 8: X-Forwarded-For header is respected
func TestRateLimiter_XForwardedFor(t *testing.T) {
	rl := NewRateLimiter(1.0, 2, "ip", true, time.Hour)
	handler := rl.Middleware(createTestHandler())

	// Request with X-Forwarded-For
	req := httptest.NewRequest("GET", "/api/test", nil)
	req.RemoteAddr = "10.0.0.1:12345" // Proxy IP
	req.Header.Set("X-Forwarded-For", "203.0.113.1, 198.51.100.1")

	// Exhaust limit for the real client IP (203.0.113.1)
	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("Request %d: got status %d, want %d", i+1, w.Code, http.StatusOK)
		}
	}

	// Next request should be rate limited
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("Excess request: got status %d, want %d", w.Code, http.StatusTooManyRequests)
	}
}

// Test 9: Concurrent requests are handled safely
func TestRateLimiter_ConcurrentSafety(t *testing.T) {
	rl := NewRateLimiter(100.0, 200, "ip", true, time.Hour)
	handler := rl.Middleware(createTestHandler())

	const numGoroutines = 50
	const requestsPerGoroutine = 10

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Launch concurrent requests from different IPs
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()

			for j := 0; j < requestsPerGoroutine; j++ {
				req := httptest.NewRequest("GET", "/api/test", nil)
				req.RemoteAddr = "192.168.1." + string(rune(id+1)) + ":12345"
				w := httptest.NewRecorder()

				handler.ServeHTTP(w, req)

				// All requests should succeed (within limit)
				if w.Code != http.StatusOK {
					t.Errorf("Goroutine %d, Request %d: got status %d, want %d",
						id, j, w.Code, http.StatusOK)
				}
			}
		}(i)
	}

	wg.Wait()
}

// Test 10: Cleanup mechanism removes inactive limiters
func TestRateLimiter_CleanupMechanism(t *testing.T) {
	// Create rate limiter with short cleanup interval
	rl := NewRateLimiter(10.0, 20, "ip", true, 100*time.Millisecond)

	// Create a limiter for an IP
	limiter1 := rl.GetLimiter("192.168.1.1")
	if limiter1 == nil {
		t.Fatal("GetLimiter returned nil")
	}

	// Verify limiter exists
	rl.mu.RLock()
	if _, exists := rl.limiters["192.168.1.1"]; !exists {
		t.Error("Limiter was not stored")
	}
	rl.mu.RUnlock()

	// Consume all tokens to make it eligible for cleanup
	for i := 0; i < 20; i++ {
		limiter1.Allow()
	}

	// Wait for cleanup interval + buffer
	time.Sleep(150 * time.Millisecond)

	// Trigger cleanup by getting a new limiter
	_ = rl.GetLimiter("192.168.1.2")

	// Verify old limiter was cleaned up
	rl.mu.RLock()
	if _, exists := rl.limiters["192.168.1.1"]; exists {
		t.Error("Inactive limiter was not cleaned up")
	}
	rl.mu.RUnlock()
}

// Test 11: maskIdentifier function
func TestMaskIdentifier(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "Short identifier",
			input: "abc",
			want:  "abc",
		},
		{
			name:  "Exactly 8 characters",
			input: "12345678",
			want:  "12345678",
		},
		{
			name:  "Long identifier",
			input: "1234567890abcdef",
			want:  "12345678...",
		},
		{
			name:  "API Key",
			input: "sk_test_1234567890abcdefghijklmnop",
			want:  "sk_test_...",
		},
		{
			name:  "Empty string",
			input: "",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := maskIdentifier(tt.input)
			if got != tt.want {
				t.Errorf("maskIdentifier(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// Test 12: GetLimiter creates new limiter if not exists
func TestRateLimiter_GetLimiter_CreatesNew(t *testing.T) {
	rl := NewRateLimiter(10.0, 20, "ip", true, time.Hour)

	identifier := "192.168.1.1"
	limiter := rl.GetLimiter(identifier)

	if limiter == nil {
		t.Fatal("GetLimiter returned nil")
	}

	// Verify limiter is stored
	rl.mu.RLock()
	entry, exists := rl.limiters[identifier]
	rl.mu.RUnlock()

	if !exists {
		t.Error("Limiter was not stored in map")
	}

	if entry.limiter != limiter {
		t.Error("Stored limiter is different from returned limiter")
	}
}

// Test 13: GetLimiter returns existing limiter
func TestRateLimiter_GetLimiter_ReturnsExisting(t *testing.T) {
	rl := NewRateLimiter(10.0, 20, "ip", true, time.Hour)

	identifier := "192.168.1.1"
	limiter1 := rl.GetLimiter(identifier)
	limiter2 := rl.GetLimiter(identifier)

	if limiter1 != limiter2 {
		t.Error("GetLimiter returned different limiters for same identifier")
	}
}

// Test 14: Allow method respects rate limit
func TestRateLimiter_Allow(t *testing.T) {
	rl := NewRateLimiter(1.0, 2, "ip", true, time.Hour)

	identifier := "192.168.1.1"

	// First 2 requests should be allowed (burst)
	for i := 0; i < 2; i++ {
		if !rl.Allow(identifier) {
			t.Errorf("Request %d was denied, want allowed", i+1)
		}
	}

	// 3rd request should be denied
	if rl.Allow(identifier) {
		t.Error("3rd request was allowed, want denied")
	}
}

// Test 15: extractIdentifier with IP strategy
func TestRateLimiter_ExtractIdentifier_IP(t *testing.T) {
	rl := NewRateLimiter(10.0, 20, "ip", true, time.Hour)

	tests := []struct {
		name       string
		remoteAddr string
		xForwarded string
		want       string
	}{
		{
			name:       "Direct connection",
			remoteAddr: "192.168.1.1:12345",
			xForwarded: "",
			want:       "192.168.1.1",
		},
		{
			name:       "With X-Forwarded-For",
			remoteAddr: "10.0.0.1:12345",
			xForwarded: "203.0.113.1, 198.51.100.1",
			want:       "203.0.113.1",
		},
		{
			name:       "IPv6 address",
			remoteAddr: "[2001:db8::1]:12345",
			xForwarded: "",
			want:       "2001:db8::1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/test", nil)
			req.RemoteAddr = tt.remoteAddr
			if tt.xForwarded != "" {
				req.Header.Set("X-Forwarded-For", tt.xForwarded)
			}

			got := rl.extractIdentifier(req)
			if got != tt.want {
				t.Errorf("extractIdentifier() = %q, want %q", got, tt.want)
			}
		})
	}
}

// Test 16: extractIdentifier with API Key strategy
func TestRateLimiter_ExtractIdentifier_APIKey(t *testing.T) {
	rl := NewRateLimiter(10.0, 20, "api_key", true, time.Hour)

	tests := []struct {
		name          string
		authorization string
		remoteAddr    string
		want          string
	}{
		{
			name:          "Bearer token",
			authorization: "Bearer sk_test_1234567890",
			remoteAddr:    "192.168.1.1:12345",
			want:          "sk_test_1234567890",
		},
		{
			name:          "No authorization header - fallback to IP",
			authorization: "",
			remoteAddr:    "192.168.1.1:12345",
			want:          "192.168.1.1",
		},
		{
			name:          "Invalid authorization format - fallback to IP",
			authorization: "InvalidFormat",
			remoteAddr:    "192.168.1.1:12345",
			want:          "192.168.1.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/test", nil)
			req.RemoteAddr = tt.remoteAddr
			if tt.authorization != "" {
				req.Header.Set("Authorization", tt.authorization)
			}

			got := rl.extractIdentifier(req)
			if got != tt.want {
				t.Errorf("extractIdentifier() = %q, want %q", got, tt.want)
			}
		})
	}
}

// Benchmark: Rate limiter performance
func BenchmarkRateLimiter_Middleware(b *testing.B) {
	rl := NewRateLimiter(1000.0, 2000, "ip", true, time.Hour)
	handler := rl.Middleware(createTestHandler())

	req := httptest.NewRequest("GET", "/api/test", nil)
	req.RemoteAddr = "192.168.1.1:12345"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
	}
}

// Benchmark: GetLimiter performance
func BenchmarkRateLimiter_GetLimiter(b *testing.B) {
	rl := NewRateLimiter(1000.0, 2000, "ip", true, time.Hour)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = rl.GetLimiter("192.168.1.1")
	}
}