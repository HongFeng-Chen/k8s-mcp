// Package tools 每个 MCP 工具一个转换器：raw args -> 应用 DTO -> 文本结果。
// 禁止在本层调用 client-go 或写业务规则，本层只做协议适配。
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	domainerrors "k8s-mcp/internal/domain/errors"
	"k8s-mcp/internal/infrastructure/observability"
)

// Tool 所有 MCP 工具适配器的统一接口。
type Tool interface {
	Definition() mcp.Tool
	Handler(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error)
}

// Decode 把 MCP raw arguments 反序列化为应用 DTO。
func Decode[T any](req *mcp.CallToolRequest) (*T, error) {
	var out T
	if len(req.Params.Arguments) == 0 {
		return &out, nil
	}
	if err := json.Unmarshal(req.Params.Arguments, &out); err != nil {
		return nil, fmt.Errorf("%w: bad tool arguments: %v", domainerrors.ErrInvalidArgument, err)
	}
	return &out, nil
}

// Text 构造成功文本结果。
func Text(s string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: s}},
	}
}

// Fail 构造错误结果：带上领域纠正提示，IsError=true，
// 让 LLM 明确知道"被拒绝了、为什么、该怎么纠正"，而不是盲目重试。
func Fail(err error) *mcp.CallToolResult {
	msg := domainerrors.ToHint(err)
	if msg != "" {
		msg += "\n原始错误：" + err.Error()
	} else {
		msg = "工具调用失败：" + err.Error()
	}
	res := Text(msg)
	res.IsError = true
	return res
}

// Register 注册工具并包一层指标埋点（调用次数、成功/失败）。
func Register(srv *mcp.Server, m *observability.Metrics, t Tool) {
	def := t.Definition()
	name := def.Name
	srv.AddTool(&def, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		start := time.Now()
		res, err := t.Handler(ctx, req)
		result := "ok"
		if err != nil || (res != nil && res.IsError) {
			result = "error"
		}
		m.Inc("k8s_mcp_tool_calls_total", map[string]string{"tool": name, "result": result})
		m.Inc("k8s_mcp_tool_duration_seconds_total", map[string]string{"tool": name})
		_ = start // 时长直方图见 observability，当前仅累计计数
		return res, err
	})
}
