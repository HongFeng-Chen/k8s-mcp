package observability

import "github.com/google/wire"

// ProviderSet 指标模块 Provider 集合（wire）。
var ProviderSet = wire.NewSet(NewMetrics)
