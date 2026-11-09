package entity

// PodLog Pod 日志领域实体。
type PodLog struct {
	Namespace string
	PodName   string
	// Lines 原始日志行（已按行切分，不含换行符）。
	Lines []string
	// Truncated 是否因字节上限被截断。
	Truncated bool
}

// Text 拼回纯文本（供 MCP TextContent 输出）。
func (l PodLog) Text() string {
	out := ""
	for _, line := range l.Lines {
		out += line + "\n"
	}
	if l.Truncated {
		out += "\n[WARN] log output truncated due to size limit\n"
	}
	return out
}
