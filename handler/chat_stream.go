package handler

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gopkg-dev/cursor2api/types"
)

// handleStreamingResponse 处理流式响应
func (h *APIHandler) handleStreamingResponse(w http.ResponseWriter, r *http.Request, req types.ChatCompletionRequest) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		h.writeError(w, http.StatusInternalServerError, "Streaming not supported", "api_error")
		return
	}

	streamID := fmt.Sprintf("chatcmpl-%d", time.Now().UnixMilli())
	created := time.Now().Unix()
	fullContent := ""
	isFirstChunk := true

	ctx := r.Context()
	contentChan, errorChan := h.cursorService.StreamChat(ctx, req.Messages, req.Model, req.ConversationID)

	for {
		select {
		case <-ctx.Done():
			log.Printf("⚠️  客户端已断开连接,终止流式响应")
			return

		case chunk, ok := <-contentChan:
			if !ok {
				// 流结束，发送最终chunk
				promptTokens := h.converter.EstimateMessagesTokens(req.Messages)
				completionTokens := h.converter.EstimateTokens(fullContent)

				finalChunk := types.ChatCompletionStreamResponse{
					ID:      streamID,
					Object:  "chat.completion.chunk",
					Created: created,
					Model:   req.Model,
					Choices: []types.ChatCompletionChoice{
						{
							Index:        0,
							Delta:        &types.ChatMessage{}, // 空 delta
							FinishReason: "stop",
						},
					},
					Usage: &types.ChatCompletionUsage{
						PromptTokens:     promptTokens,
						CompletionTokens: completionTokens,
						TotalTokens:      promptTokens + completionTokens,
					},
				}

				h.writeSSE(w, finalChunk)
				if _, err := fmt.Fprintf(w, "data: [DONE]\n\n"); err != nil {
					log.Printf("❌ 写入 [DONE] 失败: %v", err)
				}
				flusher.Flush()

				log.Printf("✅ [流式] OpenAI 响应完成")
				log.Printf("  └─ 总内容长度: %d 字符", len(fullContent))
				log.Printf("  └─ Prompt Tokens: %d", promptTokens)
				log.Printf("  └─ Completion Tokens: %d", completionTokens)
				log.Printf("  └─ 完整响应: %s", fullContent)
				return
			}

			fullContent += chunk

			// 构建 delta
			var delta *types.ChatMessage
			if isFirstChunk {
				// 第一个 chunk 包含 role
				delta = &types.ChatMessage{
					Role:    "assistant",
					Content: chunk,
				}
				isFirstChunk = false
			} else {
				// 后续 chunk 只有 content
				delta = &types.ChatMessage{
					Content: chunk,
				}
			}

			streamChunk := types.ChatCompletionStreamResponse{
				ID:      streamID,
				Object:  "chat.completion.chunk",
				Created: created,
				Model:   req.Model,
				Choices: []types.ChatCompletionChoice{
					{
						Index:        0,
						Delta:        delta,
						FinishReason: "",
					},
				},
			}

			h.writeSSE(w, streamChunk)
			flusher.Flush()

		case err := <-errorChan:
			if err != nil {
				log.Printf("❌ 流式请求错误: %v", err)
				errorChunk := types.ErrorResponse{
					Error: types.ErrorDetail{
						Message: err.Error(),
						Type:    "api_error",
					},
				}
				h.writeSSE(w, errorChunk)
				flusher.Flush()
				return
			}
		}
	}
}

// handleNonStreamingResponse 处理非流式响应
func (h *APIHandler) handleNonStreamingResponse(w http.ResponseWriter, r *http.Request, req types.ChatCompletionRequest) {
	ctx := r.Context()

	content, err := h.cursorService.Chat(ctx, req.Messages, req.Model, req.ConversationID)
	if err != nil {
		if ctx.Err() != nil {
			log.Printf("⚠️  客户端已断开连接: %v", ctx.Err())
			return
		}
		log.Printf("❌ API 调用失败: %v", err)
		h.writeError(w, http.StatusInternalServerError, err.Error(), "api_error")
		return
	}

	promptTokens := h.converter.EstimateMessagesTokens(req.Messages)
	completionTokens := h.converter.EstimateTokens(content)

	response := types.ChatCompletionResponse{
		ID:      fmt.Sprintf("chatcmpl-%d", time.Now().UnixMilli()),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   req.Model,
		Choices: []types.ChatCompletionChoice{
			{
				Index: 0,
				Message: &types.ChatMessage{
					Role:    "assistant",
					Content: content,
				},
				FinishReason: "stop",
			},
		},
		Usage: types.ChatCompletionUsage{
			PromptTokens:     promptTokens,
			CompletionTokens: completionTokens,
			TotalTokens:      promptTokens + completionTokens,
		},
	}

	log.Printf("✅ [非流式] OpenAI 响应完成")
	log.Printf("  └─ 内容长度: %d 字符", len(content))
	log.Printf("  └─ Prompt Tokens: %d", promptTokens)
	log.Printf("  └─ Completion Tokens: %d", completionTokens)
	log.Printf("  └─ 响应内容: %s", content)

	h.writeJSON(w, http.StatusOK, response)
}
