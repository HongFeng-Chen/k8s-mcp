package vo

import (
	"fmt"
	"strings"

	domainerrors "k8s-mcp/internal/domain/errors"
)

// ImageRef 镜像引用值对象，带仓库白名单校验。
// 白名单模式（registry/repo 前缀匹配）：
//   - "docker.io/ourteam"       允许 docker.io/ourteam 下所有镜像
//   - "docker.io/ourteam/app"   只允许该仓库
//   - "*"                       允许任意镜像（不推荐）
type ImageRef struct {
	value string
}

// NewImageRef 按白名单校验镜像引用。
func NewImageRef(s string, whitelist []string) (ImageRef, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return ImageRef{}, fmt.Errorf("%w: image is empty", domainerrors.ErrInvalidArgument)
	}
	if !hasExplicitRegistry(s) {
		// 形如 "nginx" 的隐式 docker.io 镜像，补全为 docker.io/library/nginx 便于前缀匹配
		s = "docker.io/library/" + s
	}
	for _, pattern := range whitelist {
		if matchImagePattern(pattern, s) {
			return ImageRef{value: s}, nil
		}
	}
	return ImageRef{}, fmt.Errorf("%w: image %q not in whitelist", domainerrors.ErrImageNotAllowed, s)
}

// String 返回镜像引用（若原始输入无 tag 则不带补全，仅规范化 registry）。
func (i ImageRef) String() string { return i.value }

// hasExplicitRegistry 判断镜像是否显式带了 registry 段：
// 形如 "nginx"（无 /）或 "nginx:1.0"（最后一段无 / 且有 :）都视为隐式 docker.io。
func hasExplicitRegistry(s string) bool {
	return strings.Contains(s, "/")
}

func matchImagePattern(pattern, image string) bool {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return false
	}
	if pattern == "*" {
		return true
	}
	return strings.HasPrefix(image, strings.TrimSuffix(pattern, "/")+"/") || image == pattern
}
