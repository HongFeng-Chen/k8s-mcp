package application

import (
	"context"
	"fmt"
	"time"

	"k8s-mcp/internal/application/dto"
	"k8s-mcp/internal/domain/entity"
	"k8s-mcp/internal/domain/repository"
	"k8s-mcp/internal/domain/service"
	"k8s-mcp/internal/domain/vo"
)

// PodUsecase Pod 相关用例（查询组：默认放行 + 审计）。
type PodUsecase struct {
	pods    repository.PodRepo
	risk    *service.RiskControlService
	perm    *service.ResourcePermissionService
	limits  LogLimits
	listLim int
	auditFn AuditFunc
}

// LogLimits 日志/列表保护参数（wire Provider 用结构体避免同类型参数冲突）。
type LogLimits struct {
	MaxTailLines int64
	MaxBytes     int64
	ListLimit    int
}

// NewPodUsecase wire Provider。
func NewPodUsecase(
	pods repository.PodRepo,
	risk *service.RiskControlService,
	perm *service.ResourcePermissionService,
	limits LogLimits,
	auditFn AuditFunc,
) *PodUsecase {
	return &PodUsecase{
		pods: pods, risk: risk, perm: perm, auditFn: auditFn,
		limits:  limits,
		listLim: limits.ListLimit,
	}
}

// ListPods 列出 Pod。
func (uc *PodUsecase) ListPods(ctx context.Context, in dto.ListPodsInput) ([]entity.Pod, error) {
	start := time.Now()
	out, err := uc.listPods(ctx, in)
	auditRecord(uc.auditFn, ctx, "list_pods", in, start, err)
	return out, err
}

func (uc *PodUsecase) listPods(ctx context.Context, in dto.ListPodsInput) ([]entity.Pod, error) {
	// 读操作：风险等级 normal，直接放行（仍做格式校验）
	if _, err := vo.NewNamespaceOrEmpty(in.Namespace); err != nil {
		return nil, err
	}
	return uc.pods.List(ctx, in.Namespace, in.LabelSelector, uc.listLim)
}

// GetPod 查看 Pod 详情（含诊断要点）。
func (uc *PodUsecase) GetPod(ctx context.Context, in dto.GetPodInput) (*entity.Pod, error) {
	start := time.Now()
	out, err := uc.getPod(ctx, in)
	auditRecord(uc.auditFn, ctx, "get_pod", in, start, err)
	return out, err
}

func (uc *PodUsecase) getPod(ctx context.Context, in dto.GetPodInput) (*entity.Pod, error) {
	ns, err := vo.NewNamespace(in.Namespace)
	if err != nil {
		return nil, err
	}
	name, err := vo.NewResourceName(in.Name)
	if err != nil {
		return nil, err
	}
	return uc.pods.Get(ctx, ns.String(), name.String())
}

// GetPodLogs 获取日志（行数/字节强制钳制）。
func (uc *PodUsecase) GetPodLogs(ctx context.Context, in dto.GetPodLogsInput) (*entity.PodLog, error) {
	start := time.Now()
	out, err := uc.getPodLogs(ctx, in)
	auditRecord(uc.auditFn, ctx, "get_pod_logs", in, start, err)
	return out, err
}

func (uc *PodUsecase) getPodLogs(ctx context.Context, in dto.GetPodLogsInput) (*entity.PodLog, error) {
	ns, err := vo.NewNamespace(in.Namespace)
	if err != nil {
		return nil, err
	}
	name, err := vo.NewResourceName(in.Name)
	if err != nil {
		return nil, err
	}
	q, err := vo.NewLogQuery(in.Container, in.TailLines, in.MaxBytes, in.Previous,
		uc.limits.MaxTailLines, uc.limits.MaxBytes)
	if err != nil {
		return nil, err
	}
	return uc.pods.GetLogs(ctx, ns.String(), name.String(), q)
}

// GetPodYAML 获取 Pod YAML（脱敏）。
func (uc *PodUsecase) GetPodYAML(ctx context.Context, in dto.GetPodInput) (string, error) {
	start := time.Now()
	ns, err := vo.NewNamespace(in.Namespace)
	if err != nil {
		auditRecord(uc.auditFn, ctx, "get_pod_yaml", in, start, err)
		return "", err
	}
	name, err := vo.NewResourceName(in.Name)
	if err != nil {
		auditRecord(uc.auditFn, ctx, "get_pod_yaml", in, start, err)
		return "", err
	}
	out, err := uc.pods.GetYAML(ctx, ns.String(), name.String())
	auditRecord(uc.auditFn, ctx, "get_pod_yaml", in, start, err)
	return out, err
}

// FormatPod 单行摘要（接入层与 usecase 共用格式）。
func FormatPod(p entity.Pod) string {
	mark := ""
	switch {
	case p.OOMKilled:
		mark = " [OOMKilled]"
	case p.RestartCount > 0:
		mark = fmt.Sprintf(" [restarts=%d]", p.RestartCount)
	}
	return fmt.Sprintf("pod/%s  phase=%s  ready=%s  node=%s%s",
		p.Namespace+"/"+p.Name, p.Phase, p.ReadySummary(), p.NodeName, mark)
}

// FormatDeployment 单行摘要。
func FormatDeployment(d entity.Deployment) string {
	status := "Ready"
	if d.IsProgressing() {
		status = "Progressing"
	}
	return fmt.Sprintf("deployment/%s  replicas=%d ready=%d updated=%d  images=%v  %s",
		d.Namespace+"/"+d.Name, d.Replicas, d.ReadyReplicas, d.UpdatedReplicas, d.Images, status)
}
