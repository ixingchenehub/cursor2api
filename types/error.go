package types

import (
	"encoding/json"
	"net/http"
)

// ErrorDetail 错误详情
type ErrorDetail struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Param   string `json:"param,omitempty"`
	Code    string `json:"code,omitempty"`
}

// ErrorResponse OpenAI 错误响应
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

// OpenAIError represents OpenAI-compatible error structure for authentication
type OpenAIError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code"`
}

// OpenAIErrorResponse wraps OpenAI error for HTTP responses
type OpenAIErrorResponse struct {
	Error OpenAIError `json:"error"`
}

// WriteJSON writes JSON response to http.ResponseWriter
func WriteJSON(w http.ResponseWriter, data interface{}) error {
	w.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(data)
}