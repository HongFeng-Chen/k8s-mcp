package vo

import (
	"fmt"

	domainerrors "k8s-mcp/internal/domain/errors"
)

// ReplicaCount 副本数校验值对象：上下限由配置注入。
type ReplicaCount struct {
	value int32
	min   int32
	max   int32
}

// NewReplicaCount 校验副本数在 [min, max] 区间内。
func NewReplicaCount(n, min, max int32) (ReplicaCount, error) {
	if max < min {
		return ReplicaCount{}, fmt.Errorf("%w: configured scale limit invalid (max %d < min %d)", domainerrors.ErrInvalidArgument, max, min)
	}
	if n > max {
		return ReplicaCount{}, fmt.Errorf("%w: replicas %d > max %d", domainerrors.ErrReplicaExceedLimit, n, max)
	}
	if n < min {
		return ReplicaCount{}, fmt.Errorf("%w: replicas %d < min %d", domainerrors.ErrReplicaBelowLimit, n, min)
	}
	return ReplicaCount{value: n, min: min, max: max}, nil
}

// Int32 返回副本数值。
func (r ReplicaCount) Int32() int32 { return r.value }

// Limits 返回配置的上下限（用于给 LLM 的提示信息）。
func (r ReplicaCount) Limits() (int32, int32) { return r.min, r.max }
