package handler

import (
	"net/http"
	"time"

	"github.com/gopkg-dev/cursor2api/types"
)

// HandleModels 处理 /v1/models 请求
func (h *APIHandler) HandleModels(w http.ResponseWriter, r *http.Request) {
	// 从配置读取模型列表
	models := make([]types.Model, len(h.config.Models))
	created := time.Now().Unix()

	for i, modelCfg := range h.config.Models {
		models[i] = types.Model{
			ID:      modelCfg.ID,
			Object:  modelCfg.Object,
			Created: created,
			OwnedBy: modelCfg.OwnedBy,
		}
	}

	response := types.ModelList{
		Object: "list",
		Data:   models,
	}

	h.writeJSON(w, http.StatusOK, response)
}

// HandleHealth 处理 /health 请求
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
