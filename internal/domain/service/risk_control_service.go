// Package service 定义领域服务：纯领域规则，不依赖 mcp / client-go / http。
package service

import (
	"fmt"

	domainerrors "k8s-mcp/internal/domain/errors"
)

// RiskLevel 操作风险等级。
type RiskLevel string

const (
	// RiskNormal 查询类，默认放行。
	RiskNormal RiskLevel = "normal"
	// RiskWarning 受控变更类，需全局写开关 + 领域校验 + 审计。
	RiskWarning RiskLevel = "warning"
	// RiskHigh 高危类，默认关闭且需人工审批。
	RiskHigh RiskLevel = "high"
)

// Operation 操作类型。
type Operation string

const (
	OpRead            Operation = "read"
	OpRestart         Operation = "restart_deployment"
	OpScale           Operation = "scale_deployment"
	OpPatchImage      Operation = "patch_deployment_image"
	OpExec            Operation = "exec_pod"
	OpDelete          Operation = "delete_resource"
	OpCreate          Operation = "create_resource"
	OpUnknownMutation Operation = "unknown_mutation"
)

// RiskControlService 风险控制领域服务：判断操作等级并做准入判定。
// RBAC 是集群侧兜底；这里是业务侧第一道（也是更严格的一道）闸门。
type RiskControlService struct{}

func NewRiskControlService() *RiskControlService { return &RiskControlService{} }

// Level 返回操作的风险等级。
func (s *RiskControlService) Level(op Operation) RiskLevel {
	switch op {
	case OpRead:
		return RiskNormal
	case OpRestart, OpScale, OpPatchImage:
		return RiskWarning
	case OpExec, OpDelete, OpCreate, OpUnknownMutation:
		return RiskHigh
	default:
		return RiskHigh // 未知操作一律按高危处理
	}
}

// Authorize 操作准入判定：
//   - normal：放行；
//   - warning：全局只读模式开启时拒绝；
//   - high：直接要求人工审批（当前版本无审批流，等于拒绝）。
func (s *RiskControlService) Authorize(op Operation, readOnlyMode bool) error {
	switch s.Level(op) {
	case RiskNormal:
		return nil
	case RiskWarning:
		if readOnlyMode {
			return domainerrors.ErrReadOnlyMode
		}
		return nil
	default:
		return domainerrors.ErrHighRiskNeedApproval
	}
}

// Describe 给 LLM 的等级说明（用于工具描述与错误提示）。
func (l RiskLevel) Describe() string {
	switch l {
	case RiskNormal:
		return "normal: read-only diagnostic operation"
	case RiskWarning:
		return "warning: controlled mutation, audited and policy-checked"
	default:
		return fmt.Sprintf("high: %s, disabled by default and requires manual approval", l)
	}
}
