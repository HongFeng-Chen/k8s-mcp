// Package application 应用层：UseCase 编排（身份校验 -> 领域校验 -> 仓储 -> 审计）。
package application

import (
	"context"
	"encoding/json"
	"time"

	"k8s-mcp/internal/infrastructure/audit"
	"k8s-mcp/pkg/shared"
)

// AuditFunc 应用层统一的审计入口（由 wire 注入 audit.Store 适配而成）。
type AuditFunc func(ctx context.Context, e audit.Entry)

// NewAuditFunc 把审计存储适配为应用层函数。
func NewAuditFunc(store audit.Store) AuditFunc {
	return func(ctx context.Context, e audit.Entry) {
		_ = store.Record(ctx, e)
	}
}

// auditRecord 审计一次工具调用。
func auditRecord(fn AuditFunc, ctx context.Context, tool string, args any, start time.Time, err error) {
	if fn == nil {
		return
	}
	entry := audit.Entry{
		Time:       start,
		Identity:   shared.IdentityFrom(ctx),
		Tool:       tool,
		Allowed:    err == nil,
		DurationMs: time.Since(start).Milliseconds(),
	}
	if args != nil {
		if data, mErr := json.Marshal(args); mErr == nil {
			entry.Args = string(data)
		}
	}
	if err != nil {
		entry.Error = err.Error()
	}
	fn(ctx, entry)
}
