package service

import "github.com/google/wire"

// ProviderSet 领域服务 Provider 集合（wire）。
// ResourcePermissionConfig 的策略参数由 cmd/server 的适配 Provider 从 config 装配，
// 保持领域层不依赖 config 包。
var ProviderSet = wire.NewSet(
	NewRiskControlService,
	NewResourcePermissionService,
)
