// Package vo 定义领域值对象。
// 值对象负责"格式合法性"校验；"策略合法性"（黑名单等）由领域服务负责。
package vo

import (
	"fmt"
	"regexp"

	domainerrors "k8s-mcp/internal/domain/errors"
)

// dns1123Label k8s 命名空间/资源名遵循 RFC 1123 Label 规范。
var dns1123Label = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)

// Namespace 命名空间值对象。
type Namespace struct{ value string }

// NewNamespace 校验并构造命名空间值对象。
func NewNamespace(s string) (Namespace, error) {
	if s == "" {
		return Namespace{}, fmt.Errorf("%w: namespace is empty", domainerrors.ErrInvalidArgument)
	}
	if len(s) > 63 || !dns1123Label.MatchString(s) {
		return Namespace{}, fmt.Errorf("%w: namespace %q is not a valid DNS-1123 label", domainerrors.ErrInvalidArgument, s)
	}
	return Namespace{value: s}, nil
}

// NewNamespaceOrEmpty 允许空值（空表示 all-namespaces）。
func NewNamespaceOrEmpty(s string) (Namespace, error) {
	if s == "" {
		return Namespace{}, nil
	}
	return NewNamespace(s)
}

// String 返回原始值。
func (n Namespace) String() string { return n.value }

// IsZero 是否为空值（调用方可能用空 ns 表示 all-namespaces）。
func (n Namespace) IsZero() bool { return n.value == "" }
