package shared

import "context"

type ctxKey int

const identityKey ctxKey = iota

// WithIdentity 把调用者身份写入请求上下文（由接入层鉴权中间件写入）。
func WithIdentity(ctx context.Context, identity string) context.Context {
	return context.WithValue(ctx, identityKey, identity)
}

// IdentityFrom 从上下文取身份，缺省 "unknown"。
func IdentityFrom(ctx context.Context) string {
	if v, ok := ctx.Value(identityKey).(string); ok && v != "" {
		return v
	}
	return "unknown"
}
