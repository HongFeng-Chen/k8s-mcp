package k8sclient

import "github.com/google/wire"

// ProviderSet k8s 客户端 Provider 集合（wire）。
var ProviderSet = wire.NewSet(NewClient)
