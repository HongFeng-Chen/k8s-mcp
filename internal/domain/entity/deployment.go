package entity

import "time"

// Deployment Deployment 领域实体。
type Deployment struct {
	Namespace         string
	Name              string
	Replicas          int32
	ReadyReplicas     int32
	AvailableReplicas int32
	UpdatedReplicas   int32
	Images            []string
	Generation        int64
	CreatedAt         time.Time
}

// Age 存活时长。
func (d Deployment) Age() time.Duration { return time.Since(d.CreatedAt) }

// IsProgressing 副本是否尚未全部就绪。
func (d Deployment) IsProgressing() bool {
	return d.ReadyReplicas < d.Replicas || d.UpdatedReplicas < d.Replicas
}
