package handler

import (
	"net/http"
	"time"

	"cursor2api/types"
)

// HandleModels handles /v1/models request
// Returns the list of available Cursor AI models
func (h *APIHandler) HandleModels(w http.ResponseWriter, r *http.Request) {
	// Return fixed list of Cursor AI models
	// These are the standard models supported by Cursor
	created := time.Now().Unix()

	models := []types.Model{
	{
		ID:      "anthropic/claude-4.5-sonnet",
		Object:  "model",
		Created: created,
		OwnedBy: "cursor",
	},
	{
		ID:      "anthropic/claude-4-sonnet",
		Object:  "model",
		Created: created,
		OwnedBy: "cursor",
	},
	{
		ID:      "anthropic/claude-opus-4.1",
		Object:  "model",
		Created: created,
		OwnedBy: "cursor",
	},
	{
		ID:      "openai/gpt-5",
		Object:  "model",
		Created: created,
		OwnedBy: "cursor",
	},
	{
		ID:      "google/gemini-2.5-pro",
		Object:  "model",
		Created: created,
		OwnedBy: "cursor",
	},
	{
		ID:      "xai/grok-4",
		Object:  "model",
		Created: created,
		OwnedBy: "cursor",
	},
	}

	response := types.ModelList{
		Object: "list",
		Data:   models,
	}

	h.writeJSON(w, http.StatusOK, response)
}

// HandleHealth handles /health request
// Returns service health status and statistics
func (h *APIHandler) HandleHealth(w http.ResponseWriter, r *http.Request) {
	stats := h.manager.GetStats()

	response := types.HealthResponse{
		Status:          "ok",
		Timestamp:       time.Now(),
		ManagerHealthy:  h.manager.IsHealthy(),
		TotalRequests:   stats["totalRequests"].(int64),
		SuccessRequests: stats["successRequests"].(int64),
		FailedRequests:  stats["failedRequests"].(int64),
		CacheHits:       stats["cacheHits"].(int64),
	}

	if paramAge, ok := stats["parameterAge"].(time.Duration); ok {
		response.ParameterAge = paramAge.String()
	}

	h.writeJSON(w, http.StatusOK, response)
}