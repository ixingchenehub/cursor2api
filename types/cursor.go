package types

// SSEEventData SSE事件数据结构
type SSEEventData struct {
	Type            string           `json:"type"`
	ID              string           `json:"id,omitempty"`
	Delta           string           `json:"delta,omitempty"`
	MessageMetadata *MessageMetadata `json:"messageMetadata,omitempty"`
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
