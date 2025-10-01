package types

// ErrorDetail 错误详情
type ErrorDetail struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Param   string `json:"param,omitempty"`
	Code    string `json:"code,omitempty"`
}

// ErrorResponse OpenAI 错误响应
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}
