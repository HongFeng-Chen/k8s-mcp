package tools

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"k8s-mcp/internal/application"
	"k8s-mcp/internal/application/dto"
)

// ExplainErrorTool explain_error：纯领域工具，把 k8s 报错翻译成自然语言 + 排查建议。
type ExplainErrorTool struct {
	uc *application.DiagnosticsUsecase
}

func NewExplainErrorTool(uc *application.DiagnosticsUsecase) *ExplainErrorTool {
	return &ExplainErrorTool{uc: uc}
}

func (t *ExplainErrorTool) Definition() mcp.Tool {
	return mcp.Tool{
		Name:        "explain_error",
		Description: "把 k8s 报错或异常状态（如 CrashLoopBackOff、ImagePullBackOff、OOMKilled、forbidden）翻译成自然语言结论与排查建议。纯本地推理，不访问集群。拿到 Pod 状态或事件后建议先调用它。",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"error": map[string]any{
					"type":        "string",
					"description": "报错原文、事件消息或 Pod 状态描述",
				},
			},
			"required": []string{"error"},
		},
	}
}

func (t *ExplainErrorTool) Handler(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	in, err := Decode[dto.ExplainErrorInput](req)
	if err != nil {
		return Fail(err), nil
	}
	return Text(t.uc.ExplainError(ctx, *in)), nil
}
