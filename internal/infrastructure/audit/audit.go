// Package audit 审计存储实现：排查 LLM 误操作事故的唯一凭证。
// 支持两种存储后端：log（JSONL 文件/标准输出）与 redis（LPUSH 列表，零依赖 RESP 客户端）。
package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"k8s-mcp/internal/infrastructure/config"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/wire"
)

// ProviderSet 审计存储 Provider 集合（wire）。
var ProviderSet = wire.NewSet(NewStore)

// NewStore 按配置构建审计存储（wire Provider）。
func NewStore(cfg *config.Config) (Store, error) {
	if !cfg.Audit.Enable {
		return nopStore{}, nil
	}
	switch cfg.Audit.Storage {
	case "redis":
		return NewRedisStore(cfg.Audit.RedisAddr, cfg.Audit.RedisKey)
	default:
		return NewLogStore(cfg.Audit.LogFile)
	}
}

// Entry 一条审计记录。
type Entry struct {
	Time       time.Time `json:"time"`
	Identity   string    `json:"identity"`        // Bearer token 对应身份
	Tool       string    `json:"tool"`            // MCP 工具名
	Args       string    `json:"args"`            // 入参 JSON（已脱敏）
	Allowed    bool      `json:"allowed"`         // 是否放行执行
	Error      string    `json:"error,omitempty"` // 拒绝/失败原因
	DurationMs int64     `json:"duration_ms"`
}

// Store 审计存储接口。
type Store interface {
	Record(ctx context.Context, e Entry) error
	Close() error
}

// LogStore JSONL 日志存储。
type LogStore struct {
	mu   sync.Mutex
	file *os.File
}

// NewLogStore 创建日志存储；path 为空时写 stdout。
func NewLogStore(path string) (*LogStore, error) {
	if path == "" {
		return &LogStore{file: os.Stdout}, nil
	}
	// 自动创建父目录，避免本地调试时因目录缺失启动失败
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create audit log dir %s: %w", dir, err)
		}
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open audit log file: %w", err)
	}
	return &LogStore{file: f}, nil
}

func (s *LogStore) Record(_ context.Context, e Entry) error {
	data, err := json.Marshal(e)
	if err != nil {
		return fmt.Errorf("marshal audit entry: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.file.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("write audit entry: %w", err)
	}
	return nil
}

func (s *LogStore) Close() error {
	if s.file != nil && s.file != os.Stdout {
		return s.file.Close()
	}
	return nil
}

// nopStore 审计关闭时的空实现。
type nopStore struct{}

func (nopStore) Record(context.Context, Entry) error { return nil }
func (nopStore) Close() error                        { return nil }
