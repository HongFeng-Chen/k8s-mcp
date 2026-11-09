// 应用装配与运行时对象：由 wire 生成代码调用的 Provider 集中在这里。
package main

import (
	"log/slog"
	"net/http"

	"k8s-mcp/internal/application"
	"k8s-mcp/internal/domain/service"
	"k8s-mcp/internal/domain/vo"
	"k8s-mcp/internal/infrastructure/audit"
	"k8s-mcp/internal/infrastructure/config"
	"k8s-mcp/internal/infrastructure/k8sclient"
	"k8s-mcp/internal/infrastructure/observability"
	"k8s-mcp/internal/presentation/mcpadapter"
)

// App 组装完成的进程级对象图。
type App struct {
	Config  *config.Config
	Handler http.Handler
	client  *k8sclient.Client
	audit   audit.Store
}

// NewApp 根路由装配（wire Provider）：
//   - /mcp     : MCP StreamableHTTP 端点（含鉴权/限流中间件）
//   - /metrics : Prometheus 指标
//   - /healthz /readyz : 探针
func NewApp(
	cfg *config.Config,
	client *k8sclient.Client,
	auditStore audit.Store,
	adapter *mcpadapter.MCPServerAdapter,
	metrics *observability.Metrics,
	rl *mcpadapter.RateLimiter,
) *App {
	mcpHandler := adapter.Handler(mcpadapter.HandlerDeps{
		Metrics: metrics,
		Authenticate: func(token string) (string, bool) {
			id := cfg.IdentityFor(token)
			return id, id != "unknown"
		},
		RateLimiter: rl,
	})

	mux := http.NewServeMux()
	mux.Handle("/mcp", mcpHandler)
	mux.Handle("/metrics", metrics.Handler())
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		// 缓存同步在启动期已完成；能响应即就绪
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ready"))
	})

	return &App{
		Config:  cfg,
		Handler: mux,
		client:  client,
		audit:   auditStore,
	}
}

// Close 优雅退出时释放资源（informer、审计存储）。
func (a *App) Close() {
	if a.client != nil {
		a.client.Close()
	}
	if a.audit != nil {
		_ = a.audit.Close()
	}
}

// ---- 以下为 config -> 领域/适配层 的参数适配 Provider ----
// （保持领域层不依赖 config 包，Kratos 中这类小适配也常放在 cmd 层）

// provideReadOnlyMode 提取全局只读开关。
func provideReadOnlyMode(cfg *config.Config) bool { return cfg.Permission.ReadOnlyMode }

// providePermissionConfig 装配资源权限领域服务的策略参数。
func providePermissionConfig(cfg *config.Config) service.ResourcePermissionConfig {
	return service.ResourcePermissionConfig{
		NamespaceBlacklist:   cfg.Permission.NamespaceBlacklist,
		MutationAllowedKinds: cfg.Permission.MutationAllowedKinds,
		ImageWhitelist:       cfg.Permission.ImageWhitelist,
		ScaleLimits: vo.ReplicaCountScaleLimit{
			MinReplicas: cfg.Permission.ScaleLimit.MinReplicas,
			MaxReplicas: cfg.Permission.ScaleLimit.MaxReplicas,
		},
	}
}

// providePodLimits 装配 Pod 用例的日志/列表保护参数。
func providePodLimits(cfg *config.Config) application.LogLimits {
	return application.LogLimits{
		MaxTailLines: cfg.Permission.LogLimits.MaxTailLines,
		MaxBytes:     cfg.Permission.LogLimits.MaxBytes,
		ListLimit:    cfg.Permission.ListLimit,
	}
}

// provideRateLimiter 按配置构建限流器（未启用时返回 nil，Allow 恒真）。
func provideRateLimiter(cfg *config.Config) *mcpadapter.RateLimiter {
	if !cfg.RateLimit.Enable {
		return nil
	}
	slog.Info("rate limit enabled", "qps", cfg.RateLimit.QPS, "burst", cfg.RateLimit.Burst)
	return mcpadapter.NewRateLimiter(cfg.RateLimit.QPS, cfg.RateLimit.Burst)
}
