package entity

// NamespaceInfo 命名空间领域实体。
type NamespaceInfo struct {
	Name   string
	Status string // Active / Terminating
	Labels map[string]string
}
