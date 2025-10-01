package types

// Model 模型信息
type Model struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

// ModelList 模型列表
type ModelList struct {
	Object string  `json:"object"`
	Data   []Model `json:"data"`
}
