// Package errors 定义领域层自定义错误。
// 这些错误是领域规则拒绝操作的唯一表达方式，
// 应用层和接入层都以此翻译为对 LLM 友好的提示文本。
package domainerrors

import "errors"

// 哨兵错误：领域规则拒绝。
var (
	// ErrInvalidArgument 入参格式非法（如 ns 名称不符合 DNS-1123）。
	ErrInvalidArgument = errors.New("invalid argument")

	// ErrForbiddenNamespace 命名空间在黑名单中，禁止操作。
	ErrForbiddenNamespace = errors.New("namespace is forbidden by policy")

	// ErrReadOnlyMode 全局只读模式开启，所有写操作被拒绝。
	ErrReadOnlyMode = errors.New("write operations are disabled: server is in read-only mode")

	// ErrHighRiskNeedApproval 高危操作需要人工审批（当前版本直接拒绝）。
	ErrHighRiskNeedApproval = errors.New("high-risk operation requires manual approval, which is not enabled on this server")

	// ErrResourceNotAllowed 资源类型不在变更白名单内。
	ErrResourceNotAllowed = errors.New("resource kind is not allowed to be mutated")

	// ErrReplicaExceedLimit 副本数超过上限。
	ErrReplicaExceedLimit = errors.New("replica count exceeds configured maximum")

	// ErrReplicaBelowLimit 副本数低于下限。
	ErrReplicaBelowLimit = errors.New("replica count is below configured minimum")

	// ErrImageNotAllowed 镜像不在白名单内。
	ErrImageNotAllowed = errors.New("image is not in the allowed registry whitelist")

	// ErrNotFound 目标资源不存在。
	ErrNotFound = errors.New("resource not found")
)

// ToHint 把领域错误翻译成给 LLM 看的纠正性提示，
// 描述越明确，LLM 越不容易反复幻觉重试。
func ToHint(err error) string {
	switch {
	case errors.Is(err, ErrForbiddenNamespace):
		return "被拒绝：该命名空间属于系统保护黑名单（如 kube-system），任何变更操作都不允许。请提示用户手动操作或换一个业务命名空间。"
	case errors.Is(err, ErrReadOnlyMode):
		return "被拒绝：当前 MCP Server 处于全局只读模式，所有写操作已被禁用。只能执行查询/诊断类操作。"
	case errors.Is(err, ErrHighRiskNeedApproval):
		return "被拒绝：该操作属于高危操作（exec/delete/create 等），默认关闭且需要人工审批，请勿重试。"
	case errors.Is(err, ErrResourceNotAllowed):
		return "被拒绝：该资源类型不在允许变更的白名单内，当前仅允许对 Deployment 做受控变更。"
	case errors.Is(err, ErrReplicaExceedLimit):
		return "被拒绝：副本数超过配置的最大上限，请缩小副本数后重试。"
	case errors.Is(err, ErrReplicaBelowLimit):
		return "被拒绝：副本数低于配置的最小下限，请调大副本数后重试。"
	case errors.Is(err, ErrImageNotAllowed):
		return "被拒绝：镜像不在白名单仓库内，只能使用配置允许的镜像仓库。"
	case errors.Is(err, ErrResourceNotAllowed):
		return "被拒绝：该资源类型不允许变更。"
	case errors.Is(err, ErrNotFound):
		return "目标资源不存在：请确认 namespace 与资源名称拼写是否正确（可用 list 类工具先确认）。"
	case errors.Is(err, ErrInvalidArgument):
		return "参数不合法：请检查参数格式（如命名空间需符合 DNS-1123 规范）后重试。"
	default:
		return ""
	}
}
