package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"k8s-mcp/internal/application"
	"k8s-mcp/internal/application/dto"
)

// ---------- list_deployments ----------

type ListDeploymentsTool struct {
	uc *application.DeploymentUsecase
}

func NewListDeploymentsTool(uc *application.DeploymentUsecase) *ListDeploymentsTool {
	return &ListDeploymentsTool{uc: uc}
}

func (t *ListDeploymentsTool) Definition() mcp.Tool {
	return mcp.Tool{
		Name:        "list_deployments",
		Description: "列出 Deployment 及副本就绪情况。不传命名空间则查全集群。",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"namespace":      map[string]any{"type": "string", "description": "k8s 命名空间，留空表示所有命名空间"},
				"label_selector": map[string]any{"type": "string", "description": "可选，k8s 标签选择器"},
			},
		},
	}
}

func (t *ListDeploymentsTool) Handler(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	in, err := Decode[dto.ListDeploymentsInput](req)
	if err != nil {
		return Fail(err), nil
	}
	deps, err := t.uc.ListDeployments(ctx, *in)
	if err != nil {
		return Fail(err), nil
	}
	if len(deps) == 0 {
		return Text("未找到匹配的 Deployment。"), nil
	}
	var b strings.Builder
	fmt.Fprintf(&b, "共 %d 个 Deployment：\n", len(deps))
	for _, d := range deps {
		b.WriteString(application.FormatDeployment(d))
		b.WriteString("\n")
	}
	return Text(b.String()), nil
}

// ---------- get_deployment ----------

type GetDeploymentTool struct {
	uc *application.DeploymentUsecase
}

func NewGetDeploymentTool(uc *application.DeploymentUsecase) *GetDeploymentTool {
	return &GetDeploymentTool{uc: uc}
}

func (t *GetDeploymentTool) Definition() mcp.Tool {
	return mcp.Tool{
		Name:        "get_deployment",
		Description: "查看单个 Deployment 详情：副本/就绪/更新数、镜像列表、generation。变更操作前建议先查看确认目标。",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"namespace": map[string]any{"type": "string", "description": "k8s 命名空间"},
				"name":      map[string]any{"type": "string", "description": "Deployment 名称"},
			},
			"required": []string{"namespace", "name"},
		},
	}
}

func (t *GetDeploymentTool) Handler(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	in, err := Decode[dto.GetDeploymentInput](req)
	if err != nil {
		return Fail(err), nil
	}
	d, err := t.uc.GetDeployment(ctx, *in)
	if err != nil {
		return Fail(err), nil
	}
	return Text(application.FormatDeployment(*d)), nil
}

// ---------- restart_deployment（受控变更） ----------

type RestartDeploymentTool struct {
	uc *application.DeploymentUsecase
}

func NewRestartDeploymentTool(uc *application.DeploymentUsecase) *RestartDeploymentTool {
	return &RestartDeploymentTool{uc: uc}
}

func (t *RestartDeploymentTool) Definition() mcp.Tool {
	return mcp.Tool{
		Name:        "restart_deployment",
		Description: "滚动重启 Deployment（通过打 restartedAt 注解触发，等同于 kubectl rollout restart，安全、不删 Pod）。属于受控变更：受命名空间黑名单、只读模式限制，且会记审计。",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"namespace": map[string]any{"type": "string", "description": "k8s 命名空间（系统命名空间被禁止）"},
				"name":      map[string]any{"type": "string", "description": "Deployment 名称"},
			},
			"required": []string{"namespace", "name"},
		},
	}
}

func (t *RestartDeploymentTool) Handler(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	in, err := Decode[dto.RestartDeploymentInput](req)
	if err != nil {
		return Fail(err), nil
	}
	if err := t.uc.RestartDeployment(ctx, *in); err != nil {
		return Fail(err), nil
	}
	return Text(fmt.Sprintf("已触发滚动重启：deployment/%s。可用 get_deployment 观察副本滚动进度。", in.Namespace+"/"+in.Name)), nil
}

// ---------- scale_deployment（受控变更） ----------

type ScaleDeploymentTool struct {
	uc *application.DeploymentUsecase
}

func NewScaleDeploymentTool(uc *application.DeploymentUsecase) *ScaleDeploymentTool {
	return &ScaleDeploymentTool{uc: uc}
}

func (t *ScaleDeploymentTool) Definition() mcp.Tool {
	return mcp.Tool{
		Name:        "scale_deployment",
		Description: "调整 Deployment 副本数。副本数必须在服务端配置的上下限内（超出会被拒绝），受命名空间黑名单、只读模式限制，且会记审计。",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"namespace": map[string]any{"type": "string", "description": "k8s 命名空间（系统命名空间被禁止）"},
				"name":      map[string]any{"type": "string", "description": "Deployment 名称"},
				"replicas":  map[string]any{"type": "integer", "description": "目标副本数"},
			},
			"required": []string{"namespace", "name", "replicas"},
		},
	}
}

func (t *ScaleDeploymentTool) Handler(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	in, err := Decode[dto.ScaleDeploymentInput](req)
	if err != nil {
		return Fail(err), nil
	}
	if err := t.uc.ScaleDeployment(ctx, *in); err != nil {
		return Fail(err), nil
	}
	return Text(fmt.Sprintf("已提交副本数变更：deployment/%s -> replicas=%d。（%s）",
		in.Namespace+"/"+in.Name, in.Replicas, t.uc.ScaleLimitHint())), nil
}

// ---------- patch_deployment_image（受控变更） ----------

type PatchDeploymentImageTool struct {
	uc *application.DeploymentUsecase
}

func NewPatchDeploymentImageTool(uc *application.DeploymentUsecase) *PatchDeploymentImageTool {
	return &PatchDeploymentImageTool{uc: uc}
}

func (t *PatchDeploymentImageTool) Definition() mcp.Tool {
	return mcp.Tool{
		Name:        "patch_deployment_image",
		Description: "更新 Deployment 指定容器的镜像。镜像必须在服务端白名单仓库内（否则拒绝），受命名空间黑名单、只读模式限制，且会记审计。",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"namespace": map[string]any{"type": "string", "description": "k8s 命名空间（系统命名空间被禁止）"},
				"name":      map[string]any{"type": "string", "description": "Deployment 名称"},
				"container": map[string]any{"type": "string", "description": "容器名（多容器 Deployment 必填）"},
				"image":     map[string]any{"type": "string", "description": "新镜像引用，如 docker.io/ourteam/app:v1.2.3"},
			},
			"required": []string{"namespace", "name", "container", "image"},
		},
	}
}

func (t *PatchDeploymentImageTool) Handler(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	in, err := Decode[dto.PatchDeploymentImageInput](req)
	if err != nil {
		return Fail(err), nil
	}
	if err := t.uc.PatchDeploymentImage(ctx, *in); err != nil {
		return Fail(err), nil
	}
	return Text(fmt.Sprintf("已提交镜像更新：deployment/%s container=%s -> %s。可用 get_deployment 观察滚动进度。",
		in.Namespace+"/"+in.Name, in.Container, in.Image)), nil
}
