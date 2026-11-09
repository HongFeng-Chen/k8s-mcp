// Package entity 定义领域实体：与 k8s API 对象解耦的纯领域模型。
// infrastructure 层负责把 client-go 对象映射到这里。
package entity

import "time"

// Pod Pod 领域实体。
type Pod struct {
	Namespace       string
	Name            string
	Phase           string // Running / Pending / Succeeded / Failed / Unknown
	NodeName        string
	PodIP           string
	RestartCount    int32
	ReadyContainers int32
	TotalContainers int32
	OOMKilled       bool
	Labels          map[string]string
	CreatedAt       time.Time
	// Reason 终止原因（如 Evicted / OOMKilled），诊断常用。
	Reason string
	// DeleteRequested 是否处于删除中。
	DeleteRequested bool
}

// ReadySummary 形如 "2/3" 的就绪概览。
func (p Pod) ReadySummary() string {
	return fmtInt(p.ReadyContainers) + "/" + fmtInt(p.TotalContainers)
}

// Age 存活时长。
func (p Pod) Age() time.Duration { return time.Since(p.CreatedAt) }
