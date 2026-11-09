package entity

import "time"

// Event k8s 事件领域实体（排查问题的第一手线索）。
type Event struct {
	Namespace     string
	Type          string // Normal / Warning
	Reason        string // 如 BackOff / FailedScheduling / Killing
	ObjectKind    string
	ObjectName    string
	Message       string
	Count         int32
	FirstSeen     time.Time
	LastSeen      time.Time
	SourceContext string
}
