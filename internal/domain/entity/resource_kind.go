package entity

// ResourceKind 资源类型（领域层口径，不直接用 k8s 的 Kind 字符串做逻辑）。
type ResourceKind string

const (
	KindPod        ResourceKind = "pod"
	KindDeployment ResourceKind = "deployment"
	KindService    ResourceKind = "service"
	KindNamespace  ResourceKind = "namespace"
	KindConfigMap  ResourceKind = "configmap"
	KindEvent      ResourceKind = "event"
)
