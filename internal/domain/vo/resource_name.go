package vo

import (
	"fmt"
	"regexp"

	domainerrors "k8s-mcp/internal/domain/errors"
)

// dns1123Subdomain 资源名（Deployment/Pod 等）遵循 RFC 1123 Subdomain 规范。
var dns1123Subdomain = regexp.MustCompile(`^[a-z0-9]([-a-z0-9.]*[a-z0-9])?$`)

// ResourceName 资源名称值对象。
type ResourceName struct{ value string }

// NewResourceName 校验并构造资源名值对象。
func NewResourceName(s string) (ResourceName, error) {
	if s == "" {
		return ResourceName{}, fmt.Errorf("%w: resource name is empty", domainerrors.ErrInvalidArgument)
	}
	if len(s) > 253 || !dns1123Subdomain.MatchString(s) {
		return ResourceName{}, fmt.Errorf("%w: resource name %q is not a valid DNS-1123 subdomain", domainerrors.ErrInvalidArgument, s)
	}
	return ResourceName{value: s}, nil
}

// String 返回原始值。
func (r ResourceName) String() string { return r.value }
