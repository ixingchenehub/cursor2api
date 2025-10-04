package handler

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"cursor2api/types"
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
	
	// Initialize tool call index counter for streaming responses (matching Python reference)
	toolCallIdx := 0

	ctx := r.Context()
	dataChan, errorChan := h.cursorService.StreamChat(ctx, req.Messages, req.Model, req.ConversationID, req.Tools)

	for {
		select {
		case <-ctx.Done():
			log.Printf("⚠️  客户端已断开连接,终止流式响应")
			return

		case data, ok := <-dataChan:
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
					log.Printf("❌ Failed to write [DONE]: %v", err)
				}
				flusher.Flush()

				// Log metadata only (no sensitive response content)
				log.Printf("✅ [Stream] OpenAI response completed")
				log.Printf("  └─ Content length: %d characters", len(fullContent))
				log.Printf("  └─ Prompt Tokens: %d", promptTokens)
				log.Printf("  └─ Completion Tokens: %d", completionTokens)
				return
			}

			// Handle tool call response - match Python reference implementation format
			if toolCall, ok := data.(types.CursorToolCall); ok {
				// Convert tool input to JSON string
				inputJSON := toolCall.ToolInput
				if inputJSON == "" {
					log.Printf("❌ Tool input is empty")
					continue
				}

				// Send tool call chunk - Critical: Match Python's streaming format
				// - Include Index field for tool call tracking
				// - Do NOT include Role in Delta (only in first text chunk)
				toolCallChunk := types.ChatCompletionStreamResponse{
					ID:      streamID,
					Object:  "chat.completion.chunk",
					Created: created,
					Model:   req.Model,
					Choices: []types.ChatCompletionChoice{
						{
							Index: 0,
							Delta: &types.ChatMessage{
								// Critical: Do NOT include Role field here (only in first text delta)
								ToolCalls: []types.ToolCall{
									{
										Index: toolCallIdx, // Critical: Include index for streaming tool calls
										ID:    toolCall.ToolID,
										Type:  "function",
										Function: types.ToolCallFunction{
											Name:      toolCall.ToolName,
											Arguments: inputJSON,
										},
									},
								},
							},
							FinishReason: "",
						},
					},
				}

				h.writeSSE(w, toolCallChunk)
				flusher.Flush()
				
				// Increment tool call index after each tool call (matching Python behavior)
				toolCallIdx++

				// Send finish chunk with tool_calls reason
				finishChunk := types.ChatCompletionStreamResponse{
					ID:      streamID,
					Object:  "chat.completion.chunk",
					Created: created,
					Model:   req.Model,
					Choices: []types.ChatCompletionChoice{
						{
							Index:        0,
							Delta:        &types.ChatMessage{},
							FinishReason: "tool_calls",
						},
					},
				}

				h.writeSSE(w, finishChunk)
				if _, err := fmt.Fprintf(w, "data: [DONE]\n\n"); err != nil {
					log.Printf("❌ Failed to write [DONE]: %v", err)
				}
				flusher.Flush()

				log.Printf("✅ [Stream] Tool call response completed")
				return
			}

			// Handle normal text chunk
			if chunk, ok := data.(string); ok {
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
			}

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

// handleNonStreamingResponse 处理非流式响应 - Supports both text and tool calls
func (h *APIHandler) handleNonStreamingResponse(w http.ResponseWriter, r *http.Request, req types.ChatCompletionRequest) {
	ctx := r.Context()

	// Chat now returns interface{} - can be string (text) or CursorToolCall (tool call)
	result, err := h.cursorService.Chat(ctx, req.Messages, req.Model, req.ConversationID, req.Tools)
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
	
	// Check if result is a tool call (matching Python's type checking logic)
	if toolCall, ok := result.(types.CursorToolCall); ok {
		// Handle tool call response - match OpenAI non-streaming format
		response := types.ChatCompletionResponse{
			ID:      fmt.Sprintf("chatcmpl-%d", time.Now().UnixMilli()),
			Object:  "chat.completion",
			Created: time.Now().Unix(),
			Model:   req.Model,
			Choices: []types.ChatCompletionChoice{
				{
					Index: 0,
					Message: &types.ChatMessage{
						Role: "assistant",
						ToolCalls: []types.ToolCall{
							{
								ID:   toolCall.ToolID,
								Type: "function",
								Function: types.ToolCallFunction{
									Name:      toolCall.ToolName,
									Arguments: toolCall.ToolInput,
								},
							},
						},
					},
					FinishReason: "tool_calls",
				},
			},
			Usage: types.ChatCompletionUsage{
				PromptTokens:     promptTokens,
				CompletionTokens: 0, // Tool calls don't consume completion tokens
				TotalTokens:      promptTokens,
			},
		}
		
		log.Printf("✅ [Non-Stream] Tool call response completed")
		log.Printf("  └─ Tool ID: %s", toolCall.ToolID)
		log.Printf("  └─ Tool Name: %s", toolCall.ToolName)
		log.Printf("  └─ Prompt Tokens: %d", promptTokens)
		
		h.writeJSON(w, http.StatusOK, response)
		return
	}
	
	// Handle normal text response
	content, ok := result.(string)
	if !ok {
		log.Printf("❌ Unexpected result type: %T", result)
		h.writeError(w, http.StatusInternalServerError, "Internal error: unexpected response type", "api_error")
		return
	}
	
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

	// Log metadata only (no sensitive response content)
	log.Printf("✅ [Non-Stream] Text response completed")
	log.Printf("  └─ Content length: %d characters", len(content))
	log.Printf("  └─ Prompt Tokens: %d", promptTokens)
	log.Printf("  └─ Completion Tokens: %d", completionTokens)

	h.writeJSON(w, http.StatusOK, response)
}
