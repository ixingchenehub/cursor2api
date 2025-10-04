package middleware

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"cursor2api/logger"
	"cursor2api/types"
	"golang.org/x/time/rate"
)

// limiterEntry wraps a rate limiter with its last access time
type limiterEntry struct {
	limiter    *rate.Limiter
	lastAccess time.Time
}

// RateLimiter manages rate limiting for requests
type RateLimiter struct {
	limiters        map[string]*limiterEntry
	mu              sync.RWMutex
	requestsPerSec  rate.Limit
	burst           int
	strategy        string
	enabled         bool
	cleanupInterval time.Duration
	lastCleanup     time.Time
}

// NewRateLimiter creates a new rate limiter instance
func NewRateLimiter(requestsPerSec float64, burst int, strategy string, enabled bool, cleanupInterval time.Duration) *RateLimiter {
	rl := &RateLimiter{
		limiters:        make(map[string]*limiterEntry),
		requestsPerSec:  rate.Limit(requestsPerSec),
		burst:           burst,
		strategy:        strategy,
		enabled:         enabled,
		cleanupInterval: cleanupInterval,
		lastCleanup:     time.Now(),
	}
	
	logger.Info("Rate limiter initialized | requests_per_sec=%.2f burst=%d strategy=%s enabled=%v cleanup_interval=%v",
		requestsPerSec, burst, strategy, enabled, cleanupInterval)
	
	return rl
}

// GetLimiter retrieves or creates a rate limiter for the given identifier
func (rl *RateLimiter) GetLimiter(identifier string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Perform periodic cleanup to prevent memory leaks
	if time.Since(rl.lastCleanup) > rl.cleanupInterval {
		rl.cleanup()
		rl.lastCleanup = time.Now()
	}

	entry, exists := rl.limiters[identifier]
	if !exists {
		entry = &limiterEntry{
			limiter:    rate.NewLimiter(rl.requestsPerSec, rl.burst),
			lastAccess: time.Now(),
		}
		rl.limiters[identifier] = entry
	} else {
		// Update last access time
		entry.lastAccess = time.Now()
	}

	return entry.limiter
}

// cleanup removes inactive limiters to prevent memory leaks
// This method assumes the caller holds the write lock
// Limiters that haven't been accessed for longer than cleanupInterval are removed
func (rl *RateLimiter) cleanup() {
	now := time.Now()
	ttl := rl.cleanupInterval
	
	// Remove entries that haven't been accessed within the TTL window
	for key, entry := range rl.limiters {
		if now.Sub(entry.lastAccess) > ttl {
			delete(rl.limiters, key)
		}
	}
}

// Allow checks if a request should be allowed for the given identifier
func (rl *RateLimiter) Allow(identifier string) bool {
	limiter := rl.GetLimiter(identifier)
	return limiter.Allow()
}

// extractIdentifier extracts the rate limit identifier from the request
// based on the configured strategy (IP or API Key)
func (rl *RateLimiter) extractIdentifier(r *http.Request) string {
	switch rl.strategy {
	case "api_key":
		// Extract API key from Authorization header
		auth := r.Header.Get("Authorization")
		if len(auth) > 7 && auth[:7] == "Bearer " {
			return auth[7:]
		}
		// Fallback to IP if no valid API key
		fallthrough
	case "ip":
		fallthrough
	default:
		// Extract client IP address
		// Check X-Forwarded-For header first (for proxied requests)
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			// Take the first IP in the chain
			ips := strings.Split(xff, ",")
			return strings.TrimSpace(ips[0])
		}
		// Check X-Real-IP header
		if xri := r.Header.Get("X-Real-IP"); xri != "" {
			return xri
		}
		// Fall back to RemoteAddr (strip port if present)
		ip := r.RemoteAddr
		// Handle IPv6 addresses: [2001:db8::1]:8080 -> 2001:db8::1
		if strings.HasPrefix(ip, "[") {
			if idx := strings.Index(ip, "]"); idx != -1 {
				return ip[1:idx]
			}
		}
		// Handle IPv4 addresses: 192.168.1.1:8080 -> 192.168.1.1
		if idx := strings.LastIndex(ip, ":"); idx != -1 {
			// Check if this is not an IPv6 address without brackets
			if strings.Count(ip, ":") == 1 {
				return ip[:idx]
			}
		}
		return ip
	}
}

// Middleware returns the rate limiting middleware handler
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip rate limiting if disabled
		if !rl.enabled {
			next.ServeHTTP(w, r)
			return
		}

		// Whitelist: /health endpoint doesn't require rate limiting
		if r.URL.Path == "/health" {
			next.ServeHTTP(w, r)
			return
		}

		// Extract identifier based on strategy
		identifier := rl.extractIdentifier(r)

		// Check rate limit
		if !rl.Allow(identifier) {
			rl.respondRateLimitExceeded(w, r, identifier)
			return
		}

		// Request allowed, proceed to next handler
		next.ServeHTTP(w, r)
	})
}

// respondRateLimitExceeded sends OpenAI-compatible 429 error response
func (rl *RateLimiter) respondRateLimitExceeded(w http.ResponseWriter, r *http.Request, identifier string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Retry-After", "60") // Suggest retry after 60 seconds
	w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%.0f", rl.requestsPerSec))
	w.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", time.Now().Add(time.Minute).Unix()))
	w.WriteHeader(http.StatusTooManyRequests)

	errResp := types.OpenAIErrorResponse{
		Error: types.OpenAIError{
			Message: "Rate limit exceeded. Please retry after 60 seconds.",
			Type:    "rate_limit_error",
			Code:    "rate_limit_exceeded",
		},
	}

	logger.Warn("Rate limit exceeded | identifier=%s client_ip=%s path=%s method=%s strategy=%s",
		maskIdentifier(identifier), getClientIP(r), r.URL.Path, r.Method, rl.strategy)

	if err := types.WriteJSON(w, errResp); err != nil {
		logger.Error("Failed to write rate limit error response | error=%v client_ip=%s", err, getClientIP(r))
	}
}

// maskIdentifier masks the identifier for logging (shows only first 8 characters)
func maskIdentifier(identifier string) string {
	if len(identifier) == 0 {
		return ""
	}
	if len(identifier) <= 8 {
		return identifier
	}
	return identifier[:8] + "..."
}