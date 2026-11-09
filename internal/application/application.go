package application

import "github.com/google/wire"

// ProviderSet 应用用例 Provider 集合（wire）。
// AuditFunc 为共享单例，被各 UseCase 注入。
var ProviderSet = wire.NewSet(
	NewAuditFunc,
	NewPodUsecase,
	NewDeploymentUsecase,
	NewClusterUsecase,
	NewDiagnosticsUsecase,
)
