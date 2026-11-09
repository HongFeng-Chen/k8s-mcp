package vo

import (
	"fmt"
)

const (
	// DefaultTailLines 默认拉取日志行数：对 LLM 上下文足够，又不会撑爆。
	DefaultTailLines = int64(100)
	// DefaultLimitBytes 默认日志最大字节数 1MiB。
	DefaultLimitBytes = int64(1 << 20)
)

// LogQuery 日志查询值对象：行数、字节上限，防止 LLM 拉全量日志打爆内存。
type LogQuery struct {
	Container  string
	TailLines  int64
	LimitBytes int64
	Previous   bool
}

// NewLogQuery 构造日志查询，强制钳制在配置上限内。
func NewLogQuery(container string, tailLines, limitBytes int64, previous bool, maxLines, maxBytes int64) (LogQuery, error) {
	if maxLines <= 0 {
		maxLines = DefaultTailLines * 10
	}
	if maxBytes <= 0 {
		maxBytes = 5 << 20 // 5MiB 兜底
	}
	if tailLines <= 0 {
		tailLines = DefaultTailLines
	}
	if tailLines > maxLines {
		tailLines = maxLines
	}
	if limitBytes <= 0 {
		limitBytes = DefaultLimitBytes
	}
	if limitBytes > maxBytes {
		limitBytes = maxBytes
	}
	return LogQuery{
		Container:  container,
		TailLines:  tailLines,
		LimitBytes: limitBytes,
		Previous:   previous,
	}, nil
}

// ValidateLines 仅供测试与提示使用。
func (q LogQuery) Summary() string {
	return fmt.Sprintf("tail=%d lines, limit=%d bytes", q.TailLines, q.LimitBytes)
}
