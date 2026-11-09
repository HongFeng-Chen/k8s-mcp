package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"k8s-mcp/internal/application"
	"k8s-mcp/internal/application/dto"
)

// ---------- list_pods ----------

type ListPodsTool struct{ uc *application.PodUsecase }

func NewListPodsTool(uc *application.PodUsecase) *ListPodsTool { return &ListPodsTool{uc: uc} }

func (t *ListPodsTool) Definition() mcp.Tool {
	return mcp.Tool{
		Name:        "list_pods",
		Description: "列出 Pod。指定命名空间时列出该 ns 下的 Pod；不传则列出全集群（受单次条数上限约束）。用于排查前的概览。",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"namespace":      map[string]any{"type": "string", "description": "k8s 命名空间，留空表示所有命名空间"},
				"label_selector": map[string]any{"type": "string", "description": "可选，k8s 标签选择器，如 app=nginx,tier=web"},
			},
		},
	}
}

func (t *ListPodsTool) Handler(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	in, err := Decode[dto.ListPodsInput](req)
	if err != nil {
		return Fail(err), nil
	}
	pods, err := t.uc.ListPods(ctx, *in)
	if err != nil {
		return Fail(err), nil
	}
	if len(pods) == 0 {
		return Text("未找到匹配的 Pod。请确认 namespace 与 label_selector 是否正确。"), nil
	}
	var b strings.Builder
	fmt.Fprintf(&b, "共 %d 个 Pod：\n", len(pods))
	for _, p := range pods {
		b.WriteString(application.FormatPod(p))
		b.WriteString("\n")
	}
	return Text(b.String()), nil
}

// ---------- get_pod ----------

type GetPodTool struct{ uc *application.PodUsecase }

func NewGetPodTool(uc *application.PodUsecase) *GetPodTool { return &GetPodTool{uc: uc} }

func (t *GetPodTool) Definition() mcp.Tool {
	return mcp.Tool{
		Name:        "get_pod",
		Description: "查看单个 Pod 详情：phase、就绪容器、重启次数、节点、是否 OOMKilled、是否删除中。诊断 Pod 状态问题的第一步。",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"namespace": map[string]any{"type": "string", "description": "k8s 命名空间"},
				"name":      map[string]any{"type": "string", "description": "Pod 名称"},
			},
			"required": []string{"namespace", "name"},
		},
	}
}

func (t *GetPodTool) Handler(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	in, err := Decode[dto.GetPodInput](req)
	if err != nil {
		return Fail(err), nil
	}
	p, err := t.uc.GetPod(ctx, *in)
	if err != nil {
		return Fail(err), nil
	}
	var b strings.Builder
	b.WriteString(application.FormatPod(*p))
	b.WriteString("\n")
	if p.DeleteRequested {
		b.WriteString("注意：该 Pod 正在删除中（Terminating）。\n")
	}
	if p.OOMKilled {
		b.WriteString("注意：容器发生过 OOMKilled，建议用 explain_error 分析并检查内存 limit。\n")
	}
	return Text(b.String()), nil
}

// ---------- get_pod_logs ----------

type GetPodLogsTool struct{ uc *application.PodUsecase }

func NewGetPodLogsTool(uc *application.PodUsecase) *GetPodLogsTool { return &GetPodLogsTool{uc: uc} }

func (t *GetPodLogsTool) Definition() mcp.Tool {
	return mcp.Tool{
		Name:        "get_pod_logs",
		Description: "获取 Pod 容器日志。默认取末尾 100 行，行数与字节受服务端上限钳制；容器崩溃时传 previous=true 查看上一次崩溃日志。禁止用于读取 Secret 明文。",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"namespace": map[string]any{"type": "string", "description": "k8s 命名空间"},
				"name":      map[string]any{"type": "string", "description": "Pod 名称"},
				"container": map[string]any{"type": "string", "description": "可选，多容器 Pod 指定容器名"},
				"tail_lines": map[string]any{
					"type": "integer", "description": "可选，取末尾 N 行，默认 100，服务端有最大值钳制",
				},
				"previous": map[string]any{
					"type": "boolean", "description": "可选，true 时读取上一次崩溃容器的日志",
				},
				"max_bytes": map[string]any{
					"type": "integer", "description": "可选，日志最大字节数，默认 1MiB，服务端有上限",
				},
			},
			"required": []string{"namespace", "name"},
		},
	}
}

func (t *GetPodLogsTool) Handler(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	in, err := Decode[dto.GetPodLogsInput](req)
	if err != nil {
		return Fail(err), nil
	}
	log, err := t.uc.GetPodLogs(ctx, *in)
	if err != nil {
		return Fail(err), nil
	}
	out := log.Text()
	if out == "" {
		return Text("日志为空（容器可能刚启动或无输出）。"), nil
	}
	return Text(out), nil
}

// ---------- get_pod_yaml ----------

type GetPodYAMLTool struct{ uc *application.PodUsecase }

func NewGetPodYAMLTool(uc *application.PodUsecase) *GetPodYAMLTool { return &GetPodYAMLTool{uc: uc} }

func (t *GetPodYAMLTool) Definition() mcp.Tool {
	return mcp.Tool{
		Name:        "get_pod_yaml",
		Description: "获取 Pod 的 YAML 定义（敏感字段如环境变量值、Secret 数据已自动脱敏为 ***REDACTED***）。用于检查调度配置、探针、挂载等。",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"namespace": map[string]any{"type": "string", "description": "k8s 命名空间"},
				"name":      map[string]any{"type": "string", "description": "Pod 名称"},
			},
			"required": []string{"namespace", "name"},
		},
	}
}

func (t *GetPodYAMLTool) Handler(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	in, err := Decode[dto.GetPodInput](req)
	if err != nil {
		return Fail(err), nil
	}
	y, err := t.uc.GetPodYAML(ctx, *in)
	if err != nil {
		return Fail(err), nil
	}
	return Text(y), nil
}
