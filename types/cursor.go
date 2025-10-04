package types

// SSEEventData SSE事件数据结构
type SSEEventData struct {
	Type            string                 `json:"type"`
	ID              string                 `json:"id,omitempty"`
	Delta           string                 `json:"delta,omitempty"`
	MessageMetadata *MessageMetadata       `json:"messageMetadata,omitempty"`
	ToolCallID      string                 `json:"toolCallId,omitempty"`
	ToolName        string                 `json:"toolName,omitempty"`
	Input           interface{} `json:"input,omitempty"`
}

// MessageMetadata 消息元数据
type MessageMetadata struct {
	Usage Usage `json:"usage"`
}

// Usage Token 使用情况
type Usage struct {
	InputTokens       int `json:"inputTokens"`
	OutputTokens      int `json:"outputTokens"`
	TotalTokens       int `json:"totalTokens"`
	CachedInputTokens int `json:"cachedInputTokens"`
}

// XIsHuManDataReq xIsHuMan 数据结构
type XIsHuManDataReq struct {
	B  int     `json:"b"`
	V  float64 `json:"v"`
	E  string  `json:"e"`
	S  string  `json:"s"`
	D  int     `json:"d"`
	VR string  `json:"vr"`
}

// ProcessResponseReq 接口返回的响应结构
type ProcessResponseReq struct {
	Success bool            `json:"success"`
	Data    XIsHuManDataReq `json:"data"`
}

// CursorChatRequest represents a Cursor chat request
type CursorChatRequest struct {
	Messages []CursorMessage `json:"messages"`
	Model    string          `json:"model"`
}

// CursorMessage represents a message in Cursor format
type CursorMessage struct {
	Role  string              `json:"role"`
	Parts []CursorMessagePart `json:"parts"`
}

// CursorMessagePart represents a part of a Cursor message
type CursorMessagePart struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// CursorToolCall represents a tool call in Cursor format
type CursorToolCall struct {
	ToolID    string `json:"tool_id"`
	ToolName  string `json:"tool_name"`
	ToolInput string `json:"tool_input"`
}
