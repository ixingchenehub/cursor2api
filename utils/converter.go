package utils

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/gopkg-dev/cursor2api/types"
)

// MessageConverter 消息格式转换器
type MessageConverter struct {
	systemPrompt string // 系统提示词
}

// NewMessageConverter 创建消息转换器
func NewMessageConverter(systemPrompt string) *MessageConverter {
	return &MessageConverter{
		systemPrompt: systemPrompt,
	}
}

// OpenAIToCursorMessages 将 OpenAI 格式的消息数组转换为 Cursor 格式
func (mc *MessageConverter) OpenAIToCursorMessages(messages []types.ChatMessage, conversationID string) []map[string]any {
	cursorMessages := make([]map[string]any, 0, len(messages))

	// 使用 conversationID 作为消息 ID 的基础,如果没有则使用时间戳
	baseID := conversationID
	if baseID == "" {
		baseID = fmt.Sprintf("chatcmpl-%d", time.Now().UnixMilli())
	}

	for i, msg := range messages {
		// 处理 system 消息 - 拼接到第一条 user 消息前面
		if msg.Role == "system" {
			continue // 稍后处理
		}

		content := msg.Content

		// 如果是第一条 user 消息,添加系统提示词
		if msg.Role == "user" && i == 0 && mc.systemPrompt != "" {
			// 检查是否存在 system 消息
			hasSystem := false
			for _, m := range messages {
				if m.Role == "system" {
					content = m.Content + "\n\n" + mc.systemPrompt + "\n\n" + msg.Content
					hasSystem = true
					break
				}
			}
			// 如果没有 system 消息,直接在 user 消息前加上提示词
			if !hasSystem {
				content = mc.systemPrompt + "\n\n" + msg.Content
			}
		}

		parts := []map[string]any{
			{
				"type": "text",
				"text": content,
			},
		}

		// 生成消息 ID: 使用会话 ID + 索引,这样同一会话的消息 ID 前缀一致
		messageID := fmt.Sprintf("msg-%s-%d", baseID, i)

		cursorMsg := map[string]any{
			"parts": parts,
			"id":    messageID,
			"role":  msg.Role,
		}

		// assistant 消息需要 metadata (如果有的话)
		if msg.Role == "assistant" {
			cursorMsg["metadata"] = map[string]any{
				"usage": map[string]any{
					"inputTokens":       0,
					"outputTokens":      mc.EstimateTokens(msg.Content),
					"totalTokens":       mc.EstimateTokens(msg.Content),
					"cachedInputTokens": 0,
				},
			}
		}

		cursorMessages = append(cursorMessages, cursorMsg)
	}

	return cursorMessages
}

// BuildCursorRequest 构建 Cursor API 请求体
func (mc *MessageConverter) BuildCursorRequest(messages []types.ChatMessage, model string, conversationID string) string {
	// 如果提供了 conversationID 就使用它,否则生成新的 ID
	requestID := conversationID
	if requestID == "" {
		requestID = fmt.Sprintf("chatcmpl-%d", time.Now().UnixMilli())
	}

	// 转换消息数组 - 传入 conversationID 以保持消息 ID 一致性
	cursorMessages := mc.OpenAIToCursorMessages(messages, requestID)

	// 构建请求体
	request := map[string]any{
		"context":  []map[string]any{{"type": "file", "content": "", "filePath": "/learn/context"}},
		"model":    model,
		"id":       requestID,
		"messages": cursorMessages,
		"trigger":  "submit-message",
	}

	// 序列化为 JSON
	jsonBytes, err := json.Marshal(request)
	if err != nil {
		return "{}"
	}

	return string(jsonBytes)
}

// EstimateTokens 估算 Token 数量
func (mc *MessageConverter) EstimateTokens(text string) int {
	return (len(text) + 3) / 4
}

// EstimateMessagesTokens 估算多条消息的 Token 总数
func (mc *MessageConverter) EstimateMessagesTokens(messages []types.ChatMessage) int {
	total := 0
	for _, msg := range messages {
		total += mc.EstimateTokens(msg.Content)
	}
	return total
}
