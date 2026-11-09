package application

import (
	"context"
	"time"

	"k8s-mcp/internal/application/dto"
	"k8s-mcp/internal/domain/entity"
	"k8s-mcp/internal/domain/repository"
	"k8s-mcp/internal/domain/service"
	"k8s-mcp/internal/domain/vo"
)

// DeploymentUsecase Deployment 用例：
// 读操作默认放行；变更操作走完整三层校验链路（风险准入 -> 黑名单/白名单 -> 仓储）。
type DeploymentUsecase struct {
	deployments repository.DeploymentRepo
	risk        *service.RiskControlService
	perm        *service.ResourcePermissionService
	readOnly    bool
	limits      LogLimits
	auditFn     AuditFunc
}

// NewDeploymentUsecase wire Provider。
func NewDeploymentUsecase(
	deployments repository.DeploymentRepo,
	risk *service.RiskControlService,
	perm *service.ResourcePermissionService,
	readOnly bool,
	limits LogLimits,
	auditFn AuditFunc,
) *DeploymentUsecase {
	return &DeploymentUsecase{
		deployments: deployments, risk: risk, perm: perm,
		readOnly: readOnly, limits: limits, auditFn: auditFn,
	}
}

// ListDeployments 列出 Deployment。
func (uc *DeploymentUsecase) ListDeployments(ctx context.Context, in dto.ListDeploymentsInput) ([]entity.Deployment, error) {
	start := time.Now()
	if _, err := vo.NewNamespaceOrEmpty(in.Namespace); err != nil {
		auditRecord(uc.auditFn, ctx, "list_deployments", in, start, err)
		return nil, err
	}
	out, err := uc.deployments.List(ctx, in.Namespace, in.LabelSelector, uc.limits.ListLimit)
	auditRecord(uc.auditFn, ctx, "list_deployments", in, start, err)
	return out, err
}

// GetDeployment 查看 Deployment 详情。
func (uc *DeploymentUsecase) GetDeployment(ctx context.Context, in dto.GetDeploymentInput) (*entity.Deployment, error) {
	start := time.Now()
	out, err := uc.getDeployment(ctx, in)
	auditRecord(uc.auditFn, ctx, "get_deployment", in, start, err)
	return out, err
}

func (uc *DeploymentUsecase) getDeployment(ctx context.Context, in dto.GetDeploymentInput) (*entity.Deployment, error) {
	ns, name, err := parseNsName(in.Namespace, in.Name)
	if err != nil {
		return nil, err
	}
	return uc.deployments.Get(ctx, ns.String(), name.String())
}

// RestartDeployment 滚动重启（受控变更，warning 级）。
func (uc *DeploymentUsecase) RestartDeployment(ctx context.Context, in dto.RestartDeploymentInput) error {
	start := time.Now()
	err := uc.restartDeployment(ctx, in)
	auditRecord(uc.auditFn, ctx, "restart_deployment", in, start, err)
	return err
}

func (uc *DeploymentUsecase) restartDeployment(ctx context.Context, in dto.RestartDeploymentInput) error {
	ns, name, err := parseNsName(in.Namespace, in.Name)
	if err != nil {
		return err
	}
	// 1) 领域层：风险准入（warning 级 -> 要求关闭只读模式）
	if err := uc.risk.Authorize(service.OpRestart, uc.readOnly); err != nil {
		return err
	}
	// 2) 领域层：资源类型白名单 + 命名空间黑名单
	if err := uc.perm.EnsureResourceAllowed(entity.KindDeployment); err != nil {
		return err
	}
	if err := uc.perm.EnsureNamespaceAllowed(ns); err != nil {
		return err
	}
	// 3) 仓储：k8s 原生 patch（restartedAt 注解）
	return uc.deployments.Restart(ctx, ns.String(), name.String())
}

// ScaleDeployment 扩缩副本（受控变更，warning 级，带上下限保护）。
func (uc *DeploymentUsecase) ScaleDeployment(ctx context.Context, in dto.ScaleDeploymentInput) error {
	start := time.Now()
	err := uc.scaleDeployment(ctx, in)
	auditRecord(uc.auditFn, ctx, "scale_deployment", in, start, err)
	return err
}

func (uc *DeploymentUsecase) scaleDeployment(ctx context.Context, in dto.ScaleDeploymentInput) error {
	ns, name, err := parseNsName(in.Namespace, in.Name)
	if err != nil {
		return err
	}
	if err := uc.risk.Authorize(service.OpScale, uc.readOnly); err != nil {
		return err
	}
	if err := uc.perm.EnsureResourceAllowed(entity.KindDeployment); err != nil {
		return err
	}
	if err := uc.perm.EnsureNamespaceAllowed(ns); err != nil {
		return err
	}
	replicas, err := uc.perm.ValidateReplicas(in.Replicas)
	if err != nil {
		return err
	}
	return uc.deployments.Scale(ctx, ns.String(), name.String(), replicas)
}

// ScaleLimitHint 暴露副本上下限提示（给 LLM 的工具描述/错误提示用）。
func (uc *DeploymentUsecase) ScaleLimitHint() string { return uc.perm.ScaleLimitHint() }

// PatchDeploymentImage 更新镜像（受控变更，warning 级，镜像白名单）。
func (uc *DeploymentUsecase) PatchDeploymentImage(ctx context.Context, in dto.PatchDeploymentImageInput) error {
	start := time.Now()
	err := uc.patchDeploymentImage(ctx, in)
	auditRecord(uc.auditFn, ctx, "patch_deployment_image", in, start, err)
	return err
}

func (uc *DeploymentUsecase) patchDeploymentImage(ctx context.Context, in dto.PatchDeploymentImageInput) error {
	ns, name, err := parseNsName(in.Namespace, in.Name)
	if err != nil {
		return err
	}
	if err := uc.risk.Authorize(service.OpPatchImage, uc.readOnly); err != nil {
		return err
	}
	if err := uc.perm.EnsureResourceAllowed(entity.KindDeployment); err != nil {
		return err
	}
	if err := uc.perm.EnsureNamespaceAllowed(ns); err != nil {
		return err
	}
	image, err := uc.perm.ValidateImage(in.Image)
	if err != nil {
		return err
	}
	return uc.deployments.PatchImage(ctx, ns.String(), name.String(), in.Container, image)
}

// GetDeploymentYAML 获取 YAML（脱敏）。
func (uc *DeploymentUsecase) GetDeploymentYAML(ctx context.Context, in dto.GetDeploymentInput) (string, error) {
	start := time.Now()
	ns, name, err := parseNsName(in.Namespace, in.Name)
	if err != nil {
		auditRecord(uc.auditFn, ctx, "get_deployment_yaml", in, start, err)
		return "", err
	}
	out, err := uc.deployments.GetYAML(ctx, ns.String(), name.String())
	auditRecord(uc.auditFn, ctx, "get_deployment_yaml", in, start, err)
	return out, err
}

func parseNsName(nsStr, nameStr string) (vo.Namespace, vo.ResourceName, error) {
	ns, err := vo.NewNamespace(nsStr)
	if err != nil {
		return vo.Namespace{}, vo.ResourceName{}, err
	}
	name, err := vo.NewResourceName(nameStr)
	if err != nil {
		return vo.Namespace{}, vo.ResourceName{}, err
	}
	return ns, name, nil
}
