package middleware

import (
	"crypto/subtle"
	"net/http"
	"strings"
	"sync"

	"cursor2api/logger"
	"cursor2api/types"
)

// APIKeyAuth handles Bearer token authentication
type APIKeyAuth struct {
	validKeys map[string]struct{}
	mu        sync.RWMutex
	enabled   bool
}

// NewAPIKeyAuth creates a new API key authentication middleware
func NewAPIKeyAuth(apiKeys []string, enabled bool) *APIKeyAuth {
	auth := &APIKeyAuth{
		validKeys: make(map[string]struct{}, len(apiKeys)),
		enabled:   enabled,
	}

	for _, key := range apiKeys {
		if key != "" {
			auth.validKeys[key] = struct{}{}
		}
	}

	logger.Info("API key authentication middleware initialized | key_count=%d enabled=%v", len(auth.validKeys), enabled)

	return auth
}

// Middleware returns the authentication middleware handler
func (a *APIKeyAuth) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip authentication if disabled
		if !a.enabled {
			next.ServeHTTP(w, r)
			return
		}

		// Whitelist: /health endpoint doesn't require authentication
		if r.URL.Path == "/health" {
			next.ServeHTTP(w, r)
			return
		}

		// Extract Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			a.respondUnauthorized(w, r, "missing_api_key", "Authorization header is required")
			return
		}

		// Parse Bearer token
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			a.respondUnauthorized(w, r, "invalid_format", "Authorization header must be 'Bearer <API_KEY>'")
			return
		}

		apiKey := parts[1]

		// Validate API key
		if !a.validateKey(apiKey) {
			logger.Warn("Invalid API key attempt | masked_key=%s client_ip=%s path=%s method=%s",
				maskAPIKey(apiKey), getClientIP(r), r.URL.Path, r.Method)

			a.respondUnauthorized(w, r, "invalid_api_key", "Invalid API key provided")
			return
		}

		// Security audit log for successful authentication
		logger.Info("API key authentication successful | masked_key=%s client_ip=%s path=%s method=%s",
			maskAPIKey(apiKey), getClientIP(r), r.URL.Path, r.Method)

		next.ServeHTTP(w, r)
	})
}

// validateKey checks if the provided API key is valid using constant-time comparison
func (a *APIKeyAuth) validateKey(key string) bool {
	a.mu.RLock()
	defer a.mu.RUnlock()

	// Use constant-time comparison to prevent timing attacks
	for validKey := range a.validKeys {
		if subtle.ConstantTimeCompare([]byte(key), []byte(validKey)) == 1 {
			return true
		}
	}

	return false
}

// ReloadKeys updates the valid API keys (supports hot reload)
func (a *APIKeyAuth) ReloadKeys(newKeys []string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.validKeys = make(map[string]struct{}, len(newKeys))
	for _, key := range newKeys {
		if key != "" {
			a.validKeys[key] = struct{}{}
		}
	}

	logger.Info("API keys reloaded successfully | key_count=%d", len(a.validKeys))
}

// respondUnauthorized sends OpenAI-compatible 401 error response
func (a *APIKeyAuth) respondUnauthorized(w http.ResponseWriter, r *http.Request, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("WWW-Authenticate", "Bearer")
	w.WriteHeader(http.StatusUnauthorized)

	errResp := types.OpenAIErrorResponse{
		Error: types.OpenAIError{
			Message: message,
			Type:    "invalid_request_error",
			Code:    code,
		},
	}

	if err := types.WriteJSON(w, errResp); err != nil {
		logger.Error("Failed to write error response | error=%v client_ip=%s", err, getClientIP(r))
	}
}

// maskAPIKey masks the API key for logging (shows only first 8 characters)
func maskAPIKey(key string) string {
	if len(key) <= 8 {
		return "****"
	}
	return key[:8] + "****"
}

// getClientIP extracts the real client IP address
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header (proxy/load balancer)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ips := strings.Split(xff, ",")
		return strings.TrimSpace(ips[0])
	}

	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// Fallback to remote address
	ip := r.RemoteAddr
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		return ip[:idx]
	}
	return ip
}