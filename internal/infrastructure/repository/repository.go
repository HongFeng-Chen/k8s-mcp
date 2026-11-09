package repository

import (
	"github.com/google/wire"

	"k8s-mcp/internal/domain/repository"
)

// ProviderSet 仓储实现 Provider 集合（wire）。
// 同时绑定 domain/repository 接口：应用层只依赖领域接口。
var ProviderSet = wire.NewSet(
	NewPodRepo,
	NewDeploymentRepo,
	NewClusterRepo,
	wire.Bind(new(repository.PodRepo), new(*PodRepo)),
	wire.Bind(new(repository.DeploymentRepo), new(*DeploymentRepo)),
	wire.Bind(new(repository.ClusterRepo), new(*ClusterRepo)),
)
