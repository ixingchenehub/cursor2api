package handler

import (
	"encoding/json"
	"fmt"
	"net/http"

	"cursor2api/types"
)

// writeJSON 写入 JSON 响应
func (h *APIHandler) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		// 已经写入了 header,无法再修改响应状态,只能记录错误
		fmt.Printf("❌ JSON 编码失败: %v\n", err)
	}
}

// writeSSE 写入 SSE 数据
func (h *APIHandler) writeSSE(w http.ResponseWriter, data interface{}) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		fmt.Printf("❌ JSON 序列化失败: %v\n", err)
		return
	}
	if _, err := fmt.Fprintf(w, "data: %s\n\n", jsonData); err != nil {
		fmt.Printf("❌ 写入 SSE 数据失败: %v\n", err)
	}
}

// writeError 写入错误响应
func (h *APIHandler) writeError(w http.ResponseWriter, status int, message, errorType string) {
	response := types.ErrorResponse{
		Error: types.ErrorDetail{
			Message: message,
			Type:    errorType,
		},
	}
	h.writeJSON(w, status, response)
}
