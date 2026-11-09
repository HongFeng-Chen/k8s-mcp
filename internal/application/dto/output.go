package dto

// ListOutput 通用列表输出（给 LLM 的结构化摘要）。
type ListOutput struct {
	Total int      `json:"total"`
	Items []string `json:"items"`
}
