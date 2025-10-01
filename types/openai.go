package types

// ChatMessage OpenAI 消息格式
type ChatMessage struct {
	Role    string `json:"role,omitempty"`    // system, user, assistant
	Content string `json:"content,omitempty"` // 消息内容
}

// ChatCompletionRequest OpenAI 聊天完成请求
type ChatCompletionRequest struct {
	Model            string                 `json:"model"`
	Messages         []ChatMessage          `json:"messages"`
	Stream           bool                   `json:"stream,omitempty"`
	Temperature      float64                `json:"temperature,omitempty"`
	TopP             float64                `json:"top_p,omitempty"`
	N                int                    `json:"n,omitempty"`
	MaxTokens        int                    `json:"max_tokens,omitempty"`
	PresencePenalty  float64                `json:"presence_penalty,omitempty"`
	FrequencyPenalty float64                `json:"frequency_penalty,omitempty"`
	Stop             []string               `json:"stop,omitempty"`
	User             string                 `json:"user,omitempty"`
	ConversationID   string                 `json:"conversation_id,omitempty"`
	Extra            map[string]interface{} `json:"-"`
}

// ChatCompletionChoice 响应选项
type ChatCompletionChoice struct {
	Index        int          `json:"index"`
	Message      *ChatMessage `json:"message,omitempty"`
	Delta        *ChatMessage `json:"delta,omitempty"`
	FinishReason string       `json:"finish_reason,omitempty"`
}

// ChatCompletionUsage Token 使用统计
type ChatCompletionUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ChatCompletionResponse OpenAI 聊天完成响应（非流式）
type ChatCompletionResponse struct {
	ID      string                   `json:"id"`
	Object  string                   `json:"object"`
	Created int64                    `json:"created"`
	Model   string                   `json:"model"`
	Choices []ChatCompletionChoice   `json:"choices"`
	Usage   ChatCompletionUsage      `json:"usage"`
}

// ChatCompletionStreamResponse OpenAI 流式响应
type ChatCompletionStreamResponse struct {
	ID      string                  `json:"id"`
	Object  string                  `json:"object"`
	Created int64                   `json:"created"`
	Model   string                  `json:"model"`
	Choices []ChatCompletionChoice  `json:"choices"`
	Usage   *ChatCompletionUsage    `json:"usage,omitempty"`
}
