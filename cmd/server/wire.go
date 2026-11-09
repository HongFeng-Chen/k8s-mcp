//go:build wireinject
// +build wireinject

// wire 注入声明文件（构建时被 //go:build wireinject 排除，
// 实际装配代码在 wire_gen.go，由 `wire ./cmd/server` 生成）。
package main

import (
	"github.com/google/wire"

	"k8s-mcp/internal/application"
	"k8s-mcp/internal/domain/service"
	"k8s-mcp/internal/infrastructure/audit"
	"k8s-mcp/internal/infrastructure/config"
	"k8s-mcp/internal/infrastructure/k8sclient"
	"k8s-mcp/internal/infrastructure/observability"
	infrarepo "k8s-mcp/internal/infrastructure/repository"
	"k8s-mcp/internal/presentation/mcpadapter"
)

// wireApp Kratos 风格的注入入口：声明 Provider 集合，由 wire 生成 wire_gen.go。
func wireApp(cfg *config.Config) (*App, func(), error) {
	panic(wire.Build(
		// 基础设施
		k8sclient.ProviderSet,
		infrarepo.ProviderSet,
		audit.ProviderSet,
		observability.ProviderSet,
		// 领域服务
		service.ProviderSet,
		// 应用用例
		application.ProviderSet,
		// MCP 适配层
		mcpadapter.ProviderSet,
		// cmd 层适配 Provider + 最终装配
		provideReadOnlyMode,
		providePermissionConfig,
		providePodLimits,
		provideRateLimiter,
		NewApp,
	))
}
