
# 工具调用功能完整实现方案

## 概述

本文档详细说明如何为 cursor2api 项目添加完整的工具调用（Tool Calling）功能支持。通过参考 Python 版本的实现，我们需要在 Go 项目中实现以下核心功能：

1. 接收并解析 OpenAI 格式的工具定义
2. 将工具定义注入到 Cursor API 请求中
3. 识别 Cursor 返回的工具调用事件
4. 将 Cursor 格式转换回 OpenAI 格式的流式响应

## 架构设计

### 数据流图

```
OpenAI Request (with tools)
    ↓
[Handler Layer] 接收请求
    ↓
[Converter Layer] 转换消息格式
    ├─→ 将 tools 注入到系统提示词
    └─→ 将 tool_calls 消息转换为文本
    ↓
[Service Layer] 调用 Cursor API
    ↓
[SSE Parser] 解析 Cursor 响应事件
    ├─→ 普通内容事件 → OpenAI delta
    └─→ tool-input-error 事件 → OpenAI tool_calls delta
    ↓
OpenAI Streaming Response
```

## 已完成的修改

### 1. 类型定义层 ✅

**文件**: `types/tool.go` (已创建)
**文件**: `types/openai.go` (已修改)

- ✅ 定义了 `Tool`, `ToolFunction`, `ToolCall` 等 OpenAI 标准类型
- ✅ 定义了 `CursorToolCall` 用于 Cursor 原生格式
- ✅ 为 `ChatMessage` 添加了 `ToolCalls` 和 `ToolCallID` 字段
- ✅ 为 `ChatCompletionRequest` 添加了 `Tools` 和 `ToolChoice` 字段

## 待实现的修改

### 2. 工具名称匹配器

**文件**: `utils/tool_matcher.go` (待创建)

**功能**: 实现工具名称的标准化和模糊匹配

```go
package utils

import "strings"

// NormalizeToolName standardizes tool names by replacing underscores with hyphens
func NormalizeToolName(name string) string {
	return strings.ReplaceAll(name, "_", "-")
}

// MatchToolName performs fuzzy matching between Cursor tool name and available tools
// Returns the matched tool name or empty string if no match found
func MatchToolName(cursorToolName string, availableTools []types.Tool) string {
	normalized := NormalizeToolName(cursorToolName)
	
	// Try exact match first
	for _, tool := range availableTools {
		if tool.Function.Name == cursorToolName {
			return tool.Function.Name
		}
	}
	
	// Try normalized match
	for _, tool := range availableTools {
		if NormalizeToolName(tool.Function.Name) == normalized {
			return tool.Function.Name
		}
	}
	
	return ""
}
```

**参考**: Python 项目 `app/utils.py:119-141`

### 3. 消息转换逻辑增强

**文件**: `utils/converter.go` (待修改)

#### 3.1 工具定义注入函数

在文件末尾添加以下函数：

```go
// injectToolsIntoSystemPrompt injects tool definitions into system prompt
func injectToolsIntoSystemPrompt(systemPrompt string, tools []types.Tool) string {
	if len(tools) == 0 {
		return systemPrompt
	}
	
	toolsJSON, err := json.MarshalIndent(tools, "", "  ")
	if err != nil {
		logger.Logger.Error("Failed to marshal tools", zap.Error(err))
		return systemPrompt
	}
	
	toolsSection := fmt.Sprintf("\n\nAvailable tools:\n```json\n%s\n```", string(toolsJSON))
	return systemPrompt + toolsSection
}

// formatToolCallsAsText converts tool_calls to human-readable text format
func formatToolCallsAsText(toolCalls []types.ToolCall) string {
	var parts []string
	for _, tc := range toolCalls {
		parts = append(parts, fmt.Sprintf("Called tool: %s with args: %s", 
			tc.Function.Name, tc.Function.Arguments))
	}
	return strings.Join(parts, "\n")
}
```

#### 3.2 修改 OpenAIToCursorMessages 函数

在现有的消息转换逻辑中添加 tool_calls 处理：

```go
func OpenAIToCursorMessages(messages []types.ChatMessage) []types.CursorMessage {
	cursorMessages := make([]types.CursorMessage, 0, len(messages))
	
	for _, msg := range messages {
		cursorMsg := types.CursorMessage{
			Role: msg.Role,
		}
		
		// Handle tool_calls messages - convert to text format
		if len(msg.ToolCalls) > 0 {
			cursorMsg.Content = formatToolCallsAsText(msg.ToolCalls)
		} else {
			cursorMsg.Content = msg.Content
		}
		
		cursorMessages = append(cursorMessages, cursorMsg)
	}
	
	return cursorMessages
}
```

#### 3.3 修改 BuildCursorRequest 函数

在构建请求时注入工具定义到系统提示词：

```go
func BuildCursorRequest(req *types.ChatCompletionRequest) *types.CursorRequest {
	messages := OpenAIToCursorMessages(req.Messages)
	
	// Inject tools into system prompt if tools are provided
	if len(req.Tools) > 0 && len(messages) > 0 {
		// Find or create system message
		hasSystem := false
		for i, msg := range messages {
			if msg.Role == "system" {
				messages[i].Content = injectToolsIntoSystemPrompt(msg.Content, req.Tools)
				hasSystem = true
				break
			}
		}
		
		// If no system message exists, create one
		if !hasSystem {
			systemMsg := types.CursorMessage{
				Role:    "system",
				Content: injectToolsIntoSystemPrompt("You are a helpful assistant.", req.Tools),
			}
			messages = append([]types.CursorMessage{systemMsg}, messages...)
		}
	}
	
	return &types.CursorRequest{
		Messages: messages,
		Model:    req.Model,
		// ... other fields
	}
}
```

**参考**: Python 项目 `main.py:157-195`

### 4. Cursor 服务层修改

**文件**: `service/cursor.go` (待修改)

#### 4.1 添加 SSE 事件类型定义

```go
// SSEEvent represents a parsed Server-Sent Event
type SSEEvent struct {
	Type    string          // event type: "content", "tool-input-error", etc.
	Data    json.RawMessage // raw JSON data
	IsError bool            // whether this is an error event
}
```

#### 4.2 修改 StreamChat 方法签名

添加一个新的 channel 用于传递工具调用信息：

```go
func (s *CursorService) StreamChat(ctx context.Context, req *types.CursorRequest) (
	<-chan []byte,      // content stream
	<-chan types.CursorToolCall,  // tool call stream
	<-chan error,       // error stream
	error,
)
```

#### 4.3 实现 SSE 事件解析

```go
// parseSSEEvent parses a single SSE event from the stream
func parseSSEEvent(line string) (*SSEEvent, error) {
	// Parse "event: xxx" and "data: xxx" format
	if strings.HasPrefix(line, "event: ") {
		eventType := strings.TrimPrefix(line, "event: ")
		return &SSEEvent{Type: eventType}, nil
	}
	
	if strings.HasPrefix(line, "data: ") {
		dataStr := strings.TrimPrefix(line, "data: ")
		return &SSEEvent{Data: json.RawMessage(dataStr)}, nil
	}
	
	return nil, nil
}

// Updated StreamChat implementation with tool call support
func (s *CursorService) StreamChat(ctx context.Context, req *types.CursorRequest) (
	<-chan []byte,
	<-chan types.CursorToolCall,
	<-chan error,
	error,
) {
	contentCh := make(chan []byte, 10)
	toolCallCh := make(chan types.CursorToolCall, 5)
	errCh := make(chan error, 1)
	
	go func() {
		defer close(contentCh)
		defer close(toolCallCh)
		defer close(errCh)
		
		scanner := bufio.NewScanner(resp.Body)
		var currentEvent SSEEvent
		
		for scanner.Scan() {
			line := scanner.Text()
			
			event, err := parseSSEEvent(line)
			if err != nil {
				errCh <- err
				return
			}
			
			if event == nil {
				continue
			}
			
			// Accumulate event data
			if event.Type != "" {
				currentEvent.Type = event.Type
			}
			if len(event.Data) > 0 {
				currentEvent.Data = event.Data
			}
			
			// Process complete event (empty line delimiter)
			if line == "" && currentEvent.Type != "" {
				if err := s.handleSSEEvent(&currentEvent, contentCh, toolCallCh); err != nil {
					errCh <- err
					return
				}
				currentEvent = SSEEvent{} // reset
			}
		}
	}()
	
	return contentCh, toolCallCh, errCh, nil
}

func (s *CursorService) handleSSEEvent(event *SSEEvent, contentCh chan<- []byte, toolCallCh chan<- types.CursorToolCall) error {
	switch event.Type {
	case "tool-input-error":
		var toolCall types.CursorToolCall
		if err := json.Unmarshal(event.Data, &toolCall); err != nil {
			return fmt.Errorf("failed to parse tool call: %w", err)
		}
		toolCallCh <- toolCall
		
	default:
		// Regular content event
		contentCh <- event.Data
	}
	
	return nil
}
```

**参考**: Python 项目 `main.py:307-323`

### 5. 流式响应处理层修改

**文件**: `handler/chat_stream.go` (待修改)

#### 5.1 修改 handleStreamingResponse 函数

```go
func (h *Handler) handleStreamingResponse(c *gin.Context, req *types.ChatCompletionRequest) {
	// ... existing setup code ...
	
	contentCh, toolCallCh, errCh, err := h.cursorService.StreamChat(ctx, cursorReq)
	if err != nil {
		// ... error handling ...
		return
	}
	
	// Track tool calls for the current response
	var toolCalls []types.ToolCall
	toolCallIndex := 0
	
	for {
		select {
		case content, ok := <-contentCh:
			if !ok {
				contentCh = nil
				continue
			}
			
			// Send regular content delta
			delta := types.ChatCompletionStreamResponse{
				ID:      responseID,
				Object:  "chat.completion.chunk",
				Created: time.Now().Unix(),
				Model:   req.Model,
				Choices: []types.ChatCompletionChoice{
					{
						Index: 0,
						Delta: &types.ChatMessage{
							Role:    "assistant",
							Content: string(content),
						},
					},
				},
			}
			
			if err := sendSSEEvent(c.Writer, delta); err != nil {
				return
			}
			
		case cursorToolCall, ok := <-toolCallCh:
			if !ok {
				toolCallCh = nil
				continue
			}
			
			// Convert Cursor tool call to OpenAI format
			toolCall := convertCursorToolCallToOpenAI(cursorToolCall, toolCallIndex, req.Tools)
			toolCalls = append(toolCalls, toolCall)
			
			// Send tool call delta
			delta := types.ChatCompletionStreamResponse{
				ID:      responseID,
				Object:  "chat.completion.chunk",
				Created: time.Now().Unix(),
				Model:   req.Model,
				Choices: []types.ChatCompletionChoice{
					{
						Index: 0,
						Delta: &types.ChatMessage{
							ToolCalls: []types.ToolCall{toolCall},
						},
					},
				},
			}
			
			if err := sendSSEEvent(c.Writer, delta); err != nil {
				return
			}
			
			toolCallIndex++
			
		case err := <-errCh:
			if err != nil {
				logger.Logger.Error("Stream error", zap.Error(err))
				sendSSEError(c.Writer, err)
				return
			}
			
		case <-ctx.Done():
			return
		}
		
		// Exit when all channels are closed
		if contentCh == nil && toolCallCh == nil {
			break
		}
	}
	
	// Send final done message
	sendSSEDone(c.Writer)
}

// convertCursorToolCallToOpenAI converts Cursor tool call format to OpenAI format
func convertCursorToolCallToOpenAI(cursorTC types.CursorToolCall, index int, availableTools []types.Tool) types.ToolCall {
	// Match tool name using fuzzy matching
	matchedName := utils.MatchToolName(cursorTC.ToolName, availableTools)
	if matchedName == "" {
		matchedName = cursorTC.ToolName
	}
	
	// Generate unique tool call ID
	toolCallID := fmt.Sprintf("call_%s_%d", cursorTC.ToolID, index)
	
	// Convert toolInput (map) to JSON string
	argsJSON, _ := json.Marshal(cursorTC.ToolInput)
	
	return types.ToolCall{
		ID:   toolCallID,
		Type: "function",
		Function: types.ToolCallFunction{
			Name:      matchedName,
			Arguments: string(argsJSON),
		},
	}
}
```

**参考**: Python 项目 `app/utils.py:238-265`

### 6. 非流式响应处理

**文件**: `handler/chat.go` (待修改)

为非流式请求添加类似的工具调用处理逻辑：

```go
func (h *Handler) handleNonStreamingResponse(c *gin.Context, req *types.ChatCompletionRequest) {
	// ... existing code ...
	
	contentCh, toolCallC