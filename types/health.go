package types

import "time"

// HealthResponse 健康检查响应
type HealthResponse struct {
	Status           string    `json:"status"`
	Timestamp        time.Time `json:"timestamp"`
	ManagerHealthy   bool      `json:"manager_healthy"`
	ParameterAge     string    `json:"parameter_age,omitempty"`
	TotalRequests    int64     `json:"total_requests"`
	SuccessRequests  int64     `json:"success_requests"`
	FailedRequests   int64     `json:"failed_requests"`
	CacheHits        int64     `json:"cache_hits"`
}
