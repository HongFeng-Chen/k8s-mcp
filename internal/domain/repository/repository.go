// Package repository 定义领域仓储接口（只定义，不实现）。
// 实现在 internal/infrastructure/repository，由应用层依赖注入。
package repository

import (
	"context"

	"k8s-mcp/internal/domain/entity"
	"k8s-mcp/internal/domain/vo"
)

// PodRepo Pod 仓储接口。
type PodRepo interface {
	// List 按 ns / 标签选择器列出 Pod；ns 为空表示所有命名空间。
	List(ctx context.Context, ns string, labelSelector string, limit int) ([]entity.Pod, error)
	// Get 精确获取单个 Pod；不存在返回 domainerrors.ErrNotFound。
	Get(ctx context.Context, ns, name string) (*entity.Pod, error)
	// GetLogs 获取容器日志（行数/字节受 vo.LogQuery 钳制）。
	GetLogs(ctx context.Context, ns, name string, q vo.LogQuery) (*entity.PodLog, error)
	// GetYAML 获取 Pod 的 YAML（已脱敏）。
	GetYAML(ctx context.Context, ns, name string) (string, error)
}

// DeploymentRepo Deployment 仓储接口（读 + 受控变更）。
type DeploymentRepo interface {
	List(ctx context.Context, ns string, labelSelector string, limit int) ([]entity.Deployment, error)
	Get(ctx context.Context, ns, name string) (*entity.Deployment, error)
	GetYAML(ctx context.Context, ns, name string) (string, error)

	// Restart 通过打 restartedAt 注解触发滚动重启（不用 delete，走 k8s 原生安全路径）。
	Restart(ctx context.Context, ns, name string) error
	// Scale 扩缩副本（副本值已在领域层校验过上下限）。
	Scale(ctx context.Context, ns, name string, replicas vo.ReplicaCount) error
	// PatchImage 更新指定容器的镜像（镜像已通过白名单校验）。
	PatchImage(ctx context.Context, ns, name, container string, image vo.ImageRef) error
}

// ClusterRepo 集群基础对象仓储接口（Namespace / Service / Event）。
type ClusterRepo interface {
	ListNamespaces(ctx context.Context) ([]entity.NamespaceInfo, error)
	ListServices(ctx context.Context, ns string, limit int) ([]entity.Service, error)
	GetService(ctx context.Context, ns, name string) (*entity.Service, error)
	GetServiceYAML(ctx context.Context, ns, name string) (string, error)
	// ListEvents 按 ns + 可选对象过滤事件；ns 为空表示所有命名空间。
	ListEvents(ctx context.Context, ns, objectName string, limit int) ([]entity.Event, error)
}
