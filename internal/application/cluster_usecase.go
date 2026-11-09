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

// ClusterUsecase Namespace / Service / Event 用例（查询组）。
type ClusterUsecase struct {
	cluster repository.ClusterRepo
	risk    *service.RiskControlService
	perm    *service.ResourcePermissionService
	limits  LogLimits
	auditFn AuditFunc
}

// NewClusterUsecase wire Provider。
func NewClusterUsecase(
	cluster repository.ClusterRepo,
	risk *service.RiskControlService,
	perm *service.ResourcePermissionService,
	limits LogLimits,
	auditFn AuditFunc,
) *ClusterUsecase {
	return &ClusterUsecase{cluster: cluster, risk: risk, perm: perm, limits: limits, auditFn: auditFn}
}

// ListNamespaces 列出命名空间。
func (uc *ClusterUsecase) ListNamespaces(ctx context.Context) ([]entity.NamespaceInfo, error) {
	start := time.Now()
	out, err := uc.cluster.ListNamespaces(ctx)
	auditRecord(uc.auditFn, ctx, "list_namespaces", nil, start, err)
	return out, err
}

// ListServices 列出 Service。
func (uc *ClusterUsecase) ListServices(ctx context.Context, in dto.ListServicesInput) ([]entity.Service, error) {
	start := time.Now()
	out, err := uc.listServices(ctx, in)
	auditRecord(uc.auditFn, ctx, "list_services", in, start, err)
	return out, err
}

func (uc *ClusterUsecase) listServices(ctx context.Context, in dto.ListServicesInput) ([]entity.Service, error) {
	if _, err := vo.NewNamespaceOrEmpty(in.Namespace); err != nil {
		return nil, err
	}
	return uc.cluster.ListServices(ctx, in.Namespace, uc.limits.ListLimit)
}

// GetEvents 查询事件（排查问题第一入口）。
func (uc *ClusterUsecase) GetEvents(ctx context.Context, in dto.GetEventsInput) ([]entity.Event, error) {
	start := time.Now()
	out, err := uc.getEvents(ctx, in)
	auditRecord(uc.auditFn, ctx, "get_events", in, start, err)
	return out, err
}

func (uc *ClusterUsecase) getEvents(ctx context.Context, in dto.GetEventsInput) ([]entity.Event, error) {
	if _, err := vo.NewNamespaceOrEmpty(in.Namespace); err != nil {
		return nil, err
	}
	if in.ObjectName != "" {
		if _, err := vo.NewResourceName(in.ObjectName); err != nil {
			return nil, err
		}
	}
	return uc.cluster.ListEvents(ctx, in.Namespace, in.ObjectName, uc.limits.ListLimit)
}

// GetServiceYAML 获取 Service YAML（脱敏）。
func (uc *ClusterUsecase) GetServiceYAML(ctx context.Context, in dto.GetDeploymentInput) (string, error) {
	start := time.Now()
	ns, name, err := parseNsName(in.Namespace, in.Name)
	if err != nil {
		auditRecord(uc.auditFn, ctx, "get_service_yaml", in, start, err)
		return "", err
	}
	out, err := uc.cluster.GetServiceYAML(ctx, ns.String(), name.String())
	auditRecord(uc.auditFn, ctx, "get_service_yaml", in, start, err)
	return out, err
}
