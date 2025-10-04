package service

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strings"

	"github.com/imroc/req/v3"
	utls "github.com/refraction-networking/utls"

	"cursor2api/models"
	"cursor2api/types"
	"cursor2api/utils"
)

// CursorService Cursor API 服务
type CursorService struct {
	manager   *models.AntiBotManager
	converter *utils.MessageConverter
	client    *req.Client
}

// NewCursorService 创建 Cursor 服务
func NewCursorService(manager *models.AntiBotManager, systemPrompt string) *CursorService {
	return &CursorService{
		manager:   manager,
		converter: utils.NewMessageConverter(systemPrompt),
		client: req.C().
			ImpersonateChrome().
			SetTLSFingerprint(utls.HelloChrome_131).
			EnableInsecureSkipVerify(),
	}
}

// Chat 非流式聊天 - Returns either text content or tool call
func (cs *CursorService) Chat(ctx context.Context, messages []types.ChatMessage, model string, conversationID string, tools []types.Tool) (interface{}, error) {
	xIsHuman, err := cs.manager.GetXIsHuman()
	if err != nil {
		log.Printf("❌ 获取认证参数失败: %v", err)
		return nil, fmt.Errorf("获取认证参数失败: %w", err)
	}

	requestBody := cs.converter.BuildCursorRequest(messages, model, conversationID, tools)

	// Log request metadata only (no sensitive content)
	log.Printf("🔵 [Non-Stream] Requesting Cursor API")
	log.Printf("  └─ Model: %s", model)
	log.Printf("  └─ ConversationID: %s", conversationID)
	log.Printf("  └─ Messages Count: %d", len(messages))
	log.Printf("  └─ Estimated Tokens: %d", cs.converter.EstimateMessagesTokens(messages))

	resp, err := cs.client.R().
		SetContext(ctx).
		SetHeaders(map[string]string{
			"referer":    "https://cursor.com/cn/learn/context",
			"x-is-human": xIsHuman,
			"x-method":   "POST",
			"x-path":     "/api/chat",
		}).
		SetBodyString(requestBody).
		Post("https://cursor.com/api/chat")

	if err != nil {
		if ctx.Err() != nil {
			log.Printf("⚠️  请求被取消: %v", ctx.Err())
			return nil, ctx.Err()
		}
		log.Printf("❌ 请求失败: %v", err)
		return nil, fmt.Errorf("请求失败: %w", err)
	}

	log.Printf("✅ 收到响应: HTTP %d", resp.StatusCode)

	if !resp.IsSuccessState() {
		responseBody := resp.String()
		log.Printf("❌ HTTP 错误: %d", resp.StatusCode)
		log.Printf("  └─ Response: %s", responseBody)
		return nil, fmt.Errorf("HTTP错误: %d", resp.StatusCode)
	}

	// Parse SSE response to check for tool calls (matching Python implementation)
	responseBody := resp.String()
	log.Printf("📥 [Non-Stream] Response received, length: %d bytes", len(responseBody))

	// Process SSE events to extract content or tool calls
	var fullContent strings.Builder
	scanner := bufio.NewScanner(strings.NewReader(responseBody))
	
	for scanner.Scan() {
		line := scanner.Text()
		
		if strings.TrimSpace(line) == "" {
			continue
		}
		
		if after, ok := strings.CutPrefix(line, "data: "); ok {
			data := after
			
			if data == "[DONE]" {
				break
			}
			
			var event types.SSEEventData
			if err := json.Unmarshal([]byte(data), &event); err != nil {
				log.Printf("⚠️  解析 SSE 事件失败: %v, data: %s", err, data)
				continue
			}
			
			// Check for tool call event - highest priority (matching Python logic)
			if event.Type == "tool-input-error" && len(tools) > 0 {
				log.Printf("🔧 [Non-Stream] Tool call event detected!")
				log.Printf("  └─ Tool Call ID: %s", event.ToolCallID)
				log.Printf("  └─ Original Tool Name: %s", event.ToolName)
				log.Printf("  └─ Input Type: %T", event.Input)
				
				// Enhanced nil check for Input field
				if event.Input == nil {
					log.Printf("⚠️  Tool input is nil, using empty JSON object")
					event.Input = "{}"
				}
				
				// First check if Input is already a string (like Python implementation)
				var inputJSON string
				if strInput, ok := event.Input.(string); ok {
					inputJSON = strInput
					log.Printf("  └─ Input already string, length: %d", len(inputJSON))
				} else {
					// Marshal to JSON if it's not a string
					inputBytes, err := json.Marshal(event.Input)
					if err != nil {
						log.Printf("❌ Failed to marshal tool input: %v", err)
						return nil, fmt.Errorf("failed to marshal tool input: %w", err)
					}
					inputJSON = string(inputBytes)
					log.Printf("  └─ Marshaled input to JSON, length: %d", len(inputJSON))
				}
				
				// Match tool name using fuzzy matching (like Python's match_tool_name)
				correctedToolName := event.ToolName
				if matchedTool := utils.FindToolByName(event.ToolName, tools); matchedTool != nil {
					correctedToolName = matchedTool.Function.Name
					if correctedToolName != event.ToolName {
						log.Printf("  └─ ✅ Tool name corrected: '%s' → '%s'", event.ToolName, correctedToolName)
					}
				} else {
					log.Printf("  └─ ⚠️  No matching tool found for '%s', using original name", event.ToolName)
				}
				
				toolCall := types.CursorToolCall{
					ToolID:    event.ToolCallID,
					ToolName:  correctedToolName,
					ToolInput: inputJSON,
				}
				
				log.Printf("🔧 [Tool Call] Detected in non-stream mode - ID: %s, Name: %s", toolCall.ToolID, toolCall.ToolName)
				
				// Return tool call immediately (matching Python's immediate return)
				return toolCall, nil
			}
			
			// Accumulate text content
			if event.Type == "text-delta" && event.Delta != "" {
				fullContent.WriteString(event.Delta)
			}
		}
	}
	
	if err := scanner.Err(); err != nil {
		log.Printf("❌ Failed to parse response: %v", err)
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	
	content := fullContent.String()
	log.Printf("📥 [Non-Stream] Text content extracted, length: %d characters", len(content))
	
	return content, nil
}

// StreamChat 流式聊天
func (cs *CursorService) StreamChat(ctx context.Context, messages []types.ChatMessage, model string, conversationID string, tools []types.Tool) (<-chan interface{}, <-chan error) {
	dataChan := make(chan interface{}, 10)
	errorChan := make(chan error, 1)

	go func() {
		defer close(dataChan)
		defer close(errorChan)

		xIsHuman, err := cs.manager.GetXIsHuman()
		if err != nil {
			log.Printf("❌ 获取认证参数失败: %v", err)
			errorChan <- fmt.Errorf("获取认证参数失败: %w", err)
			return
		}

		requestBody := cs.converter.BuildCursorRequest(messages, model, conversationID, tools)

		// Log request metadata only (no sensitive content)
		log.Printf("🟢 [Stream] Requesting Cursor API")
		log.Printf("  └─ Model: %s", model)
		log.Printf("  └─ ConversationID: %s", conversationID)
		log.Printf("  └─ Messages Count: %d", len(messages))
		log.Printf("  └─ Estimated Tokens: %d", cs.converter.EstimateMessagesTokens(messages))

		resp, err := cs.client.R().
			SetContext(ctx).
			SetHeaders(map[string]string{
				"referer":    "https://cursor.com/cn/learn/context",
				"x-is-human": xIsHuman,
				"x-method":   "POST",
				"x-path":     "/api/chat",
			}).
			SetBodyString(requestBody).
			DisableAutoReadResponse().
			Post("https://cursor.com/api/chat")

		if err != nil {
			if ctx.Err() != nil {
				log.Printf("⚠️  请求被取消: %v", ctx.Err())
				errorChan <- ctx.Err()
				return
			}
			log.Printf("❌ 请求失败: %v", err)
			errorChan <- fmt.Errorf("请求失败: %w", err)
			return
		}

		log.Printf("✅ Response received: HTTP %d", resp.StatusCode)

		if !resp.IsSuccessState() {
			log.Printf("❌ HTTP error: %d", resp.StatusCode)
			errorChan <- fmt.Errorf("HTTP error: %d", resp.StatusCode)
			return
		}

		log.Printf("📥 [Stream] Starting to receive SSE stream...")
		chunkCount := 0
		totalBytes := 0

		// 创建可中断的 Reader
		bodyReader := &contextReader{
			ctx:    ctx,
			reader: resp.Body,
		}
		defer func() {
			_ = resp.Body.Close()
		}()

		scanner := bufio.NewScanner(bodyReader)
		for scanner.Scan() {
			line := scanner.Text()

			if strings.TrimSpace(line) == "" {
				continue
			}

			if after, ok := strings.CutPrefix(line, "data: "); ok {
				data := after

				if data == "[DONE]" {
					log.Printf("✅ [流式] 接收完成,共 %d 个 chunk", chunkCount)
					break
				}

				var event types.SSEEventData
				if err := json.Unmarshal([]byte(data), &event); err != nil {
					log.Printf("⚠️  解析 SSE 事件失败: %v, data: %s", err, data)
					continue
				}

				// Handle tool call event - match Python reference implementation
				if event.Type == "tool-input-error" && len(tools) > 0 {
					log.Printf("🔧 [Stream] Tool call event detected!")
					log.Printf("  └─ Tool Call ID: %s", event.ToolCallID)
					log.Printf("  └─ Original Tool Name: %s", event.ToolName)
					log.Printf("  └─ Input Type: %T", event.Input)
					
					// Enhanced nil check for Input field
					if event.Input == nil {
						log.Printf("⚠️  Tool input is nil, using empty JSON object")
						event.Input = "{}"
					}
					
					// First check if Input is already a string (like Python implementation)
					var inputJSON string
					if strInput, ok := event.Input.(string); ok {
						inputJSON = strInput
						log.Printf("  └─ Input already string, length: %d", len(inputJSON))
					} else {
						// Marshal to JSON if it's not a string
						inputBytes, err := json.Marshal(event.Input)
						if err != nil {
							log.Printf("❌ Failed to marshal tool input: %v", err)
							errorChan <- fmt.Errorf("failed to marshal tool input: %w", err)
							return
						}
						inputJSON = string(inputBytes)
						log.Printf("  └─ Marshaled input to JSON, length: %d", len(inputJSON))
					}

					// Match tool name using fuzzy matching (like Python's match_tool_name)
					correctedToolName := event.ToolName
					if matchedTool := utils.FindToolByName(event.ToolName, tools); matchedTool != nil {
						correctedToolName = matchedTool.Function.Name
						if correctedToolName != event.ToolName {
							log.Printf("  └─ ✅ Tool name corrected: '%s' → '%s'", event.ToolName, correctedToolName)
						}
					} else {
						log.Printf("  └─ ⚠️  No matching tool found for '%s', using original name", event.ToolName)
					}

					toolCall := types.CursorToolCall{
						ToolID:    event.ToolCallID,
						ToolName:  correctedToolName,
						ToolInput: inputJSON,
					}

					log.Printf("🔧 [Tool Call] Detected - ID: %s, Name: %s", toolCall.ToolID, toolCall.ToolName)

					// Send tool call and immediately close stream (critical: like Python's return)
					select {
					case <-ctx.Done():
						log.Printf("⚠️  Context cancelled while sending tool call")
						return
					case dataChan <- toolCall:
						log.Printf("✅ [Tool Call] Sent successfully, closing stream immediately")
						// Critical: Return immediately after sending tool call, don't continue processing
						return
					}
				}

				if event.Type == "text-delta" && event.Delta != "" {
					chunkCount++
					totalBytes += len(event.Delta)

					// Send chunk without logging sensitive content
					select {
					case <-ctx.Done():
						log.Printf("⚠️  发送 chunk 时检测到客户端取消")
						return
					case dataChan <- event.Delta:
						// 发送成功
					}
				}
			}
		}

		// Check scanner errors
		if err := scanner.Err(); err != nil {
			// If error is due to context cancellation, return directly
			if ctx.Err() != nil {
				log.Printf("⚠️  Client cancelled during stream reading: %v", ctx.Err())
				return
			}
			log.Printf("❌ Failed to read response stream: %v", err)
			errorChan <- fmt.Errorf("failed to read response stream: %w", err)
			return
		}

		log.Printf("✅ [Stream] Completed - Chunks: %d, Total bytes: %d", chunkCount, totalBytes)
	}()

	return dataChan, errorChan
}

// contextReader 包装 io.Reader,使其能响应 context 取消
type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

// Read 实现 io.Reader 接口,在每次读取前检查 context 状态
func (cr *contextReader) Read(p []byte) (n int, err error) {
	// 检查 context 是否已取消
	select {
	case <-cr.ctx.Done():
		log.Printf("⚠️  contextReader 检测到客户端取消")
		return 0, cr.ctx.Err()
	default:
	}

	// 执行实际读取
	return cr.reader.Read(p)
}

