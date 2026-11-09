package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"k8s-mcp/internal/application"
	"k8s-mcp/internal/application/dto"
)

// ---------- list_namespaces ----------

type ListNamespacesTool struct{ uc *application.ClusterUsecase }

func NewListNamespacesTool(uc *application.ClusterUsecase) *ListNamespacesTool {
	return &ListNamespacesTool{uc: uc}
}

func (t *ListNamespacesTool) Definition() mcp.Tool {
	return mcp.Tool{
		Name:        "list_namespaces",
		Description: "列出集群命名空间。诊断第一步：先确认业务命名空间名称。系统命名空间（kube-system 等）对变更操作始终被禁止。",
		InputSchema: map[string]any{"type": "object", "properties": map[string]any{}},
	}
}

func (t *ListNamespacesTool) Handler(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	nss, err := t.uc.ListNamespaces(ctx)
	if err != nil {
		return Fail(err), nil
	}
	var b strings.Builder
	fmt.Fprintf(&b, "共 %d 个命名空间：\n", len(nss))
	for _, ns := range nss {
		fmt.Fprintf(&b, "ns/%s  status=%s\n", ns.Name, ns.Status)
	}
	return Text(b.String()), nil
}

// ---------- list_services ----------

type ListServicesTool struct{ uc *application.ClusterUsecase }

func NewListServicesTool(uc *application.ClusterUsecase) *ListServicesTool {
	return &ListServicesTool{uc: uc}
}

func (t *ListServicesTool) Definition() mcp.Tool {
	return mcp.Tool{
		Name:        "list_services",
		Description: "列出 Service 及端口/选择器。排查服务发现问题时使用（selector 是否匹配 Pod 标签）。",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"namespace": map[string]any{"type": "string", "description": "k8s 命名空间，留空表示所有命名空间"},
			},
		},
	}
}

func (t *ListServicesTool) Handler(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	in, err := Decode[dto.ListServicesInput](req)
	if err != nil {
		return Fail(err), nil
	}
	svcs, err := t.uc.ListServices(ctx, *in)
	if err != nil {
		return Fail(err), nil
	}
	if len(svcs) == 0 {
		return Text("未找到匹配的 Service。"), nil
	}
	var b strings.Builder
	fmt.Fprintf(&b, "共 %d 个 Service：\n", len(svcs))
	for _, s := range svcs {
		ports := make([]string, 0, len(s.Ports))
		for _, p := range s.Ports {
			ports = append(ports, fmt.Sprintf("%d->%s/%s", p.Port, p.TargetPort, p.Protocol))
		}
		fmt.Fprintf(&b, "service/%s  type=%s  clusterIP=%s  ports=[%s]  selector=%s\n",
			s.Namespace+"/"+s.Name, s.Type, s.ClusterIP, strings.Join(ports, ","), s.SelectorString())
	}
	return Text(b.String()), nil
}

// ---------- get_events ----------

type GetEventsTool struct{ uc *application.ClusterUsecase }

func NewGetEventsTool(uc *application.ClusterUsecase) *GetEventsTool {
	return &GetEventsTool{uc: uc}
}

func (t *GetEventsTool) Definition() mcp.Tool {
	return mcp.Tool{
		Name:        "get_events",
		Description: "查询 k8s 事件（Warning 事件最有诊断价值）。可按命名空间与对象名过滤。Pod 异常时优先查事件。",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"namespace":   map[string]any{"type": "string", "description": "k8s 命名空间，留空表示所有命名空间"},
				"object_name": map[string]any{"type": "string", "description": "可选，只看某个对象（如 Pod 名）的事件"},
			},
		},
	}
}

func (t *GetEventsTool) Handler(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	in, err := Decode[dto.GetEventsInput](req)
	if err != nil {
		return Fail(err), nil
	}
	events, err := t.uc.GetEvents(ctx, *in)
	if err != nil {
		return Fail(err), nil
	}
	if len(events) == 0 {
		return Text("未查询到相关事件。"), nil
	}
	var b strings.Builder
	fmt.Fprintf(&b, "共 %d 条事件（Warning 优先排查）：\n", len(events))
	for _, e := range events {
		fmt.Fprintf(&b, "[%s] %s/%s  reason=%s  count=%d  %s\n",
			e.Type, strings.ToLower(e.ObjectKind), e.ObjectName, e.Reason, e.Count, e.Message)
	}
	return Text(b.String()), nil
}

// ---------- get_resource_yaml ----------

type GetResourceYAMLTool struct {
	pods        *application.PodUsecase
	deployments *application.DeploymentUsecase
	cluster     *application.ClusterUsecase
}

func NewGetResourceYAMLTool(pods *application.PodUsecase, deps *application.DeploymentUsecase, cluster *application.ClusterUsecase) *GetResourceYAMLTool {
	return &GetResourceYAMLTool{pods: pods, deployments: deps, cluster: cluster}
}

func (t *GetResourceYAMLTool) Definition() mcp.Tool {
	return mcp.Tool{
		Name:        "get_resource_yaml",
		Description: "获取资源的 YAML 定义（已脱敏：env 明文值、Secret 数据等替换为 ***REDACTED***）。kind 支持 pod / deployment / service。",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"namespace": map[string]any{"type": "string", "description": "k8s 命名空间"},
				"kind":      map[string]any{"type": "string", "description": "资源类型：pod / deployment / service"},
				"name":      map[string]any{"type": "string", "description": "资源名称"},
			},
			"required": []string{"namespace", "kind", "name"},
		},
	}
}

func (t *GetResourceYAMLTool) Handler(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	in, err := Decode[dto.GetResourceYAMLInput](req)
	if err != nil {
		return Fail(err), nil
	}
	switch strings.ToLower(in.Kind) {
	case "pod":
		y, err := t.pods.GetPodYAML(ctx, dto.GetPodInput{Namespace: in.Namespace, Name: in.Name})
		return yamlResult(y, err)
	case "deployment":
		y, err := t.deployments.GetDeploymentYAML(ctx, dto.GetDeploymentInput{Namespace: in.Namespace, Name: in.Name})
		return yamlResult(y, err)
	case "service":
		y, err := t.cluster.GetServiceYAML(ctx, dto.GetDeploymentInput{Namespace: in.Namespace, Name: in.Name})
		return yamlResult(y, err)
	default:
		return Fail(fmt.Errorf("不支持的资源类型 %q，仅支持 pod / deployment / service", in.Kind)), nil
	}
}

func yamlResult(y string, err error) (*mcp.CallToolResult, error) {
	if err != nil {
		return Fail(err), nil
	}
	return Text(y), nil
}
