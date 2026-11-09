// Package dto 应用层入参/出参：纯数据载体，不含行为。
// 接入层负责把 MCP raw args 反序列化到这里；应用层不感知 MCP 协议。
package dto

// ListPodsInput list_pods 入参。
type ListPodsInput struct {
	Namespace     string `json:"namespace"`
	LabelSelector string `json:"label_selector"`
}

// GetPodInput get_pod 入参。
type GetPodInput struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

// GetPodLogsInput get_pod_logs 入参。
type GetPodLogsInput struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Container string `json:"container"`
	TailLines int64  `json:"tail_lines"`
	Previous  bool   `json:"previous"`
	MaxBytes  int64  `json:"max_bytes"`
}

// ListDeploymentsInput list_deployments 入参。
type ListDeploymentsInput struct {
	Namespace     string `json:"namespace"`
	LabelSelector string `json:"label_selector"`
}

// GetDeploymentInput get_deployment 入参。
type GetDeploymentInput struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

// GetResourceYAMLInput get_resource_yaml 入参。
type GetResourceYAMLInput struct {
	Namespace string `json:"namespace"`
	Kind      string `json:"kind"` // pod | deployment | service
	Name      string `json:"name"`
}

// ListServicesInput list_services 入参。
type ListServicesInput struct {
	Namespace string `json:"namespace"`
}

// GetEventsInput get_events 入参。
type GetEventsInput struct {
	Namespace  string `json:"namespace"`
	ObjectName string `json:"object_name"`
}

// RestartDeploymentInput restart_deployment 入参。
type RestartDeploymentInput struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

// ScaleDeploymentInput scale_deployment 入参。
type ScaleDeploymentInput struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Replicas  int32  `json:"replicas"`
}

// PatchDeploymentImageInput patch_deployment_image 入参。
type PatchDeploymentImageInput struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Container string `json:"container"`
	Image     string `json:"image"`
}

// ExplainErrorInput explain_error 入参。
type ExplainErrorInput struct {
	Error string `json:"error"`
}
