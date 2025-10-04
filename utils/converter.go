package utils

import (
	"cursor2api/config"
	"cursor2api/logger"
	"cursor2api/types"
	"encoding/json"
	"fmt"
)

// MessageConverter handles OpenAI to Cursor message conversion
type MessageConverter struct {
	systemPrompt string
}

// NewMessageConverter creates a new message converter
func NewMessageConverter(systemPrompt string) *MessageConverter {
	return &MessageConverter{
		systemPrompt: systemPrompt,
	}
}

// BuildCursorRequest builds a Cursor API request body
func (mc *MessageConverter) BuildCursorRequest(messages []types.ChatMessage, model string, conversationID string, tools []types.Tool) string {
	cursorReq, err := ConvertOpenAIToCursorRequest(&types.ChatCompletionRequest{
		Messages: messages,
		Model:    model,
		Tools:    tools,
	})
	if err != nil {
		logger.Error("Failed to convert request: %v", err)
		return ""
	}

	requestBody, err := json.Marshal(cursorReq)
	if err != nil {
		logger.Error("Failed to marshal request: %v", err)
		return ""
	}

	return string(requestBody)
}

// EstimateMessagesTokens estimates the token count for messages
func (mc *MessageConverter) EstimateMessagesTokens(messages []types.ChatMessage) int {
	totalChars := 0
	for _, msg := range messages {
		content := extractTextFromContent(msg.Content)
		totalChars += len(content)
	}
	// Rough estimation: 1 token ≈ 4 characters for English, 1 token ≈ 2 characters for Chinese
	return totalChars / 3
}

// EstimateTokens estimates the token count for a single text string
func (mc *MessageConverter) EstimateTokens(text string) int {
	// Rough estimation: 1 token ≈ 4 characters for English, 1 token ≈ 2 characters for Chinese
	return len(text) / 3
}

// ConvertOpenAIToCursorRequest converts OpenAI format request to Cursor format
func ConvertOpenAIToCursorRequest(req *types.ChatCompletionRequest) (*types.CursorChatRequest, error) {
	messages, err := convertMessages(req.Messages, req.Tools)
	if err != nil {
		return nil, fmt.Errorf("failed to convert messages: %w", err)
	}

	cursorReq := &types.CursorChatRequest{
		Messages: messages,
		Model:    req.Model,
	}

	return cursorReq, nil
}

// convertMessages converts OpenAI messages to Cursor format with tool injection
func convertMessages(messages []types.ChatMessage, tools []types.Tool) ([]types.CursorMessage, error) {
	// CRITICAL: Inject tools into system prompt if function calling is enabled
	if config.GlobalConfig.Cursor.EnableFunctionCalling && len(tools) > 0 {
		injectToolsIntoSystemPrompt(messages, tools)
	}

	cursorMessages := make([]types.CursorMessage, 0, len(messages))

	for _, msg := range messages {
		// Handle tool_calls in assistant messages
		if config.GlobalConfig.Cursor.EnableFunctionCalling && msg.Role == "assistant" && len(msg.ToolCalls) > 0 {
			toolCallsJSON, err := json.Marshal(msg.ToolCalls)
			if err != nil {
				logger.Error("Failed to marshal tool_calls: %v", err)
				continue
			}

			cursorMsg := types.CursorMessage{
				Role: msg.Role,
				Parts: []types.CursorMessagePart{
					{
						Type: "text",
						Text: fmt.Sprintf("tool_calls: %s", string(toolCallsJSON)),
					},
				},
			}
			cursorMessages = append(cursorMessages, cursorMsg)
			continue
		}

		// Handle tool response messages
		if config.GlobalConfig.Cursor.EnableFunctionCalling && msg.Role == "tool" && msg.ToolCallID != "" {
			cursorMsg := types.CursorMessage{
				Role: "user",
				Parts: []types.CursorMessagePart{
					{
						Type: "text",
						Text: fmt.Sprintf("%s: tool_call_id: %s %v", msg.Role, msg.ToolCallID, msg.Content),
					},
				},
			}
			cursorMessages = append(cursorMessages, cursorMsg)
			continue
		}

		// Regular message handling
		text := extractTextFromContent(msg.Content)
		if text == "" && msg.Role == "system" {
			continue
		}

		cursorMsg := types.CursorMessage{
			Role: msg.Role,
			Parts: []types.CursorMessagePart{
				{
					Type: "text",
					Text: text,
				},
			},
		}
		cursorMessages = append(cursorMessages, cursorMsg)
	}

	return cursorMessages, nil
}

// extractTextFromContent extracts text from various content types
func extractTextFromContent(content interface{}) string {
	if content == nil {
		return ""
	}

	switch v := content.(type) {
	case string:
		return v
	case []interface{}:
		var text string
		for _, item := range v {
			if contentMap, ok := item.(map[string]interface{}); ok {
				if contentMap["type"] == "text" {
					if textVal, ok := contentMap["text"].(string); ok {
						text += textVal
					}
				}
			}
		}
		return text
	default:
		return fmt.Sprintf("%v", v)
	}
}

// injectToolsIntoSystemPrompt injects tool definitions into system message
// CRITICAL: This must match Python's exact implementation (main.py:232-236)
func injectToolsIntoSystemPrompt(messages []types.ChatMessage, tools []types.Tool) {
	if len(tools) == 0 {
		return
	}

	// CRITICAL: Match Python's exact serialization format
	// Python: tools = [tool.model_dump_json() for tool in request.tools]
	// This creates a list of JSON strings, not a list of objects
	toolJSONStrings := make([]string, 0, len(tools))
	for _, tool := range tools {
		toolJSON, err := json.Marshal(tool)
		if err != nil {
			logger.Error("Failed to marshal single tool to JSON: %v, tool_name: %s", err, tool.Function.Name)
			continue
		}
		toolJSONStrings = append(toolJSONStrings, string(toolJSON))
	}

	// Now serialize the array of JSON strings
	toolsArrayJSON, err := json.Marshal(toolJSONStrings)
	if err != nil {
		logger.Error("Failed to marshal tools array to JSON: %v", err)
		return
	}

	// CRITICAL: Inject TWO separate prompts exactly as Python does
	// First injection: tool definitions
	firstPrompt := fmt.Sprintf("你可用的工具: %s", string(toolsArrayJSON))
	injectSinglePrompt(messages, firstPrompt)

	// Second injection: usage instruction
	secondPrompt := "不允许使用tool_calls: xxxx调用工具，请使用原生的工具调用方法"
	injectSinglePrompt(messages, secondPrompt)

	logger.Debug("Tools injected into system prompt, tool_count: %d, first_prompt_preview: %s",
		len(tools), firstPrompt[:min(100, len(firstPrompt))])
}

// injectSinglePrompt injects a single prompt into the first system message
func injectSinglePrompt(messages []types.ChatMessage, prompt string) {
	// Find system message
	systemMsgIndex := -1
	for i, msg := range messages {
		if msg.Role == "system" {
			systemMsgIndex = i
			break
		}
	}

	if systemMsgIndex == -1 {
		// No system message exists, create one at the beginning
		systemMsg := types.ChatMessage{
			Role:    "system",
			Content: prompt,
		}
		// Prepend to messages slice
		messages = append([]types.ChatMessage{systemMsg}, messages...)
	} else {
		// System message exists, append to its content
		currentContent := extractTextFromContent(messages[systemMsgIndex].Content)
		messages[systemMsgIndex].Content = currentContent + "\n" + prompt
	}
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}