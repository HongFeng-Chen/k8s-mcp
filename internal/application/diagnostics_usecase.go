package application

import (
	"context"
	"time"

	"k8s-mcp/internal/application/dto"
	"k8s-mcp/internal/domain/service"
)

// DiagnosticsUsecase explain_error 用例：纯领域处理，不调 apiserver。
type DiagnosticsUsecase struct {
	auditFn AuditFunc
}

func NewDiagnosticsUsecase(auditFn AuditFunc) *DiagnosticsUsecase {
	return &DiagnosticsUsecase{auditFn: auditFn}
}

// ExplainError 把 k8s 报错/状态翻译成自然语言 + 排查建议。
func (uc *DiagnosticsUsecase) ExplainError(ctx context.Context, in dto.ExplainErrorInput) string {
	start := time.Now()
	out := service.ExplainError(in.Error)
	// explain_error 是纯本地推理，恒成功，仍记审计保证调用链完整
	auditRecord(uc.auditFn, ctx, "explain_error", in, start, nil)
	return out
}
