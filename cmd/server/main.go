// k8s-mcp-server：集群内 Addon 形态的 MCP Server，
// 作为 LLM 控制 K8s 的受控代理（MCP = 命令翻译 + 安全校验层）。
//
// 依赖注入采用 google/wire（Kratos 思路）：
//   - cmd/server/wire.go     : wireinject 声明（不参与构建）
//   - cmd/server/wire_gen.go : `wire ./cmd/server` 生成的实际装配代码
//   - 各层 provider.go       : ProviderSet
package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"k8s-mcp/internal/infrastructure/config"
)

func main() {
	configPath := flag.String("config", "configs/config.yaml", "配置文件路径")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("load config", "err", err)
		os.Exit(1)
	}

	// wire 生成：wireApp(cfg) -> (*App, cleanup func, error)
	app, cleanup, err := wireApp(cfg)
	if err != nil {
		slog.Error("build application", "err", err)
		os.Exit(1)
	}
	defer cleanup()
	defer app.Close() // 停止 informer、关闭审计存储

	mode := "read-only"
	if !cfg.Permission.ReadOnlyMode {
		mode = "read-write (controlled mutations enabled)"
	}
	slog.Info("k8s-mcp-server starting",
		"listen", cfg.Server.Listen,
		"mode", mode,
		"audit", cfg.Audit.Storage,
		"cache", cfg.Cache.Enable,
		"rate_limit", cfg.RateLimit.Enable,
	)

	srv := &http.Server{
		Addr:              cfg.Server.Listen,
		Handler:           app.Handler,
		ReadHeaderTimeout: 10 * time.Second,
		// SSE/Streamable 长连接交给 MCP SDK 管理；这里只兜底极不活跃的连接
		IdleTimeout: 10 * time.Minute,
	}

	// 优雅退出
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()
		slog.Info("shutting down...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("http server exited", "err", err)
		os.Exit(1)
	}
	slog.Info("k8s-mcp-server stopped")
}
