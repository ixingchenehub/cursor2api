package types

// Tool represents a function tool that can be called by the model
type Tool struct {
	Type     string       `json:"type"`
	Function FunctionDef  `json:"function"`
}

// FunctionDef defines a function's metadata
type FunctionDef struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
}

// ToolCall represents a function call made by the model
type ToolCall struct {
	Index    int                 `json:"index,omitempty"`    // Index for streaming tool calls
	ID       string              `json:"id"`
	Type     string              `json:"type"`
	Function ToolCallFunction    `json:"function"`
}

// ToolCallFunction represents the function details in a tool call
type ToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}