package service

import (
	"strings"

	"k8s-mcp/internal/domain/entity"
	domainerrors "k8s-mcp/internal/domain/errors"
	"k8s-mcp/internal/domain/vo"
)

// ResourcePermissionConfig 资源权限领域服务所需的策略配置（由 config 层注入）。
type ResourcePermissionConfig struct {
	NamespaceBlacklist []string
	// MutationAllowedKinds 允许变更的资源类型（v1：仅 Deployment）。
	MutationAllowedKinds []string
	// ImageWhitelist 镜像仓库白名单。
	ImageWhitelist []string
	// ScaleLimits 副本上下限。
	ScaleLimits vo.ReplicaCountScaleLimit
}

// ResourcePermissionService 资源权限领域服务：
// 命名空间黑名单、资源类型白名单、镜像白名单、副本上下限。
// RBAC 拦不住 "LLM 幻觉删除 kube-system"，这类业务规则全部收口在这里。
type ResourcePermissionService struct {
	blacklist      map[string]bool
	allowedKinds   map[string]bool
	imageWhitelist []string
	scaleLimits    vo.ReplicaCountScaleLimit
}

func NewResourcePermissionService(cfg ResourcePermissionConfig) *ResourcePermissionService {
	blacklist := make(map[string]bool, len(cfg.NamespaceBlacklist))
	for _, ns := range cfg.NamespaceBlacklist {
		blacklist[strings.TrimSpace(ns)] = true
	}
	kinds := make(map[string]bool, len(cfg.MutationAllowedKinds))
	for _, k := range cfg.MutationAllowedKinds {
		kinds[strings.ToLower(strings.TrimSpace(k))] = true
	}
	return &ResourcePermissionService{
		blacklist:      blacklist,
		allowedKinds:   kinds,
		imageWhitelist: cfg.ImageWhitelist,
		scaleLimits:    cfg.ScaleLimits,
	}
}

// ParseNamespace 解析并校验命名空间格式（读操作用，不做黑名单拦截）。
func (s *ResourcePermissionService) ParseNamespace(ns string) (vo.Namespace, error) {
	return vo.NewNamespace(ns)
}

// EnsureNamespaceAllowed 变更操作前校验命名空间是否允许写。
func (s *ResourcePermissionService) EnsureNamespaceAllowed(ns vo.Namespace) error {
	if s.blacklist[ns.String()] {
		return domainerrors.ErrForbiddenNamespace
	}
	return nil
}

// EnsureResourceAllowed 校验资源类型是否允许变更（v1 仅 Deployment）。
func (s *ResourcePermissionService) EnsureResourceAllowed(kind entity.ResourceKind) error {
	if !s.allowedKinds[strings.ToLower(string(kind))] {
		return domainerrors.ErrResourceNotAllowed
	}
	return nil
}

// ValidateReplicas 校验副本数在配置上下限内。
func (s *ResourcePermissionService) ValidateReplicas(n int32) (vo.ReplicaCount, error) {
	return vo.NewReplicaCount(n, s.scaleLimits.MinReplicas, s.scaleLimits.MaxReplicas)
}

// ValidateImage 校验镜像在白名单内。
func (s *ResourcePermissionService) ValidateImage(image string) (vo.ImageRef, error) {
	return vo.NewImageRef(image, s.imageWhitelist)
}

// ScaleLimitHint 给 LLM 的副本限制提示。
func (s *ResourcePermissionService) ScaleLimitHint() string {
	return "allowed replica range: [" +
		i32(s.scaleLimits.MinReplicas) + ", " + i32(s.scaleLimits.MaxReplicas) + "]"
}
