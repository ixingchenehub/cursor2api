package handler

import (
	"encoding/json"
	"log"
	"net/http"

	"cursor2api/types"
)

// HandleChatCompletions 处理 /v1/chat/completions 请求
func (h *APIHandler) HandleChatCompletions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeError(w, http.StatusMethodNotAllowed, "Method not allowed", "invalid_request_error")
		return
	}

	var req types.ChatCompletionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("❌ 无效的 JSON: %v", err)
		h.writeError(w, http.StatusBadRequest, "Invalid JSON", "invalid_request_error")
		return
	}

	if len(req.Messages) == 0 {
		log.Printf("❌ messages 字段为空")
		h.writeError(w, http.StatusBadRequest, "messages field is required and must be a non-empty array", "invalid_request_error")
		return
	}

	if req.Model == "" {
		req.Model = "anthropic/claude-opus-4.1"
	}

	// Log request metadata only (no sensitive message content)
	log.Printf("📩 Received OpenAI request")
	log.Printf("  └─ Model: %s", req.Model)
	log.Printf("  └─ Messages Count: %d", len(req.Messages))
	log.Printf("  └─ Stream: %v", req.Stream)
	log.Printf("  └─ Tools Count: %d", len(req.Tools))
	log.Printf("  └─ ConversationID: %s", req.ConversationID)

	if req.Stream {
		h.handleStreamingResponse(w, r, req)
	} else {
		h.handleNonStreamingResponse(w, r, req)
	}
}
