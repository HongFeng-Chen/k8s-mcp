// Package config 加载并校验 config.yaml。
// 所有安全开关（只读模式、黑名单、白名单、副本上限、审计）都从这里进入系统。
package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config 根配置。
type Config struct {
	Server     ServerConfig     `yaml:"server"`
	Permission PermissionConfig `yaml:"permission"`
	Cache      CacheConfig      `yaml:"cache"`
	RateLimit  RateLimitConfig  `yaml:"rate_limit"`
	Audit      AuditConfig      `yaml:"audit"`
	Approve    ApproveConfig    `yaml:"approve"`
}

// ServerConfig HTTP/MCP 服务配置。
type ServerConfig struct {
	Listen string `yaml:"listen"`
	// Kubeconfig 本地调试用 kubeconfig 路径；为空时使用 InClusterConfig。
	Kubeconfig string `yaml:"kubeconfig"`
	// AuthTokens 接入身份列表（Bearer Token -> 身份名，写入审计）。
	AuthTokens []AuthToken `yaml:"auth_tokens"`
}

// IdentityFor 返回 token 对应身份；未知 token 返回 "unknown"。
func (c *Config) IdentityFor(token string) string {
	for _, t := range c.Server.AuthTokens {
		if t.Token != "" && t.Token == token {
			return t.Name
		}
	}
	return "unknown"
}

// AuthToken 一个接入身份：token -> 名称（审计用）。
type AuthToken struct {
	Token string `yaml:"token"`
	Name  string `yaml:"name"`
}

// PermissionConfig 权限模型配置。
type PermissionConfig struct {
	// ReadOnlyMode 全局只读开关：true 时所有写工具不注册且领域层二次拦截。
	ReadOnlyMode bool `yaml:"read_only_mode"`
	// NamespaceBlacklist 系统命名空间黑名单。
	NamespaceBlacklist []string `yaml:"namespace_blacklist"`
	// MutationAllowedKinds 允许变更的资源类型（v1 固定建议 deployment）。
	MutationAllowedKinds []string `yaml:"mutation_allowed_kinds"`
	ImageWhitelist       []string `yaml:"image_whitelist"`
	ScaleLimit           struct {
		MaxReplicas int32 `yaml:"max_replicas"`
		MinReplicas int32 `yaml:"min_replicas"`
	} `yaml:"scale_limit"`
	// LogLimits 日志拉取保护。
	LogLimits struct {
		MaxTailLines int64 `yaml:"max_tail_lines"`
		MaxBytes     int64 `yaml:"max_bytes"`
	} `yaml:"log_limits"`
	// ListLimit 单次 list 返回给 LLM 的最大条数。
	ListLimit int `yaml:"list_limit"`
}

// CacheConfig Informer 缓存配置。
type CacheConfig struct {
	// Enable 开启后 list/get 类查询走本地 informer 缓存，减轻 apiserver 压力。
	Enable      bool `yaml:"enable"`
	ResyncHours int  `yaml:"resync_hours"`
}

// RateLimitConfig 工具调用限流（防 LLM 循环调用打挂集群）。
type RateLimitConfig struct {
	Enable bool    `yaml:"enable"`
	QPS    float64 `yaml:"qps"`
	Burst  int     `yaml:"burst"`
}

// AuditConfig 审计配置。
type AuditConfig struct {
	Enable  bool   `yaml:"enable"`
	Storage string `yaml:"storage"` // log | redis
	// LogFile storage=log 时的输出文件；为空写 stdout。
	LogFile string `yaml:"log_file"`
	// RedisAddr storage=redis 时的连接地址。
	RedisAddr string `yaml:"redis_addr"`
	RedisKey  string `yaml:"redis_key"`
}

// ApproveConfig 审批流配置（Phase3 预留，当前高危操作直接拒绝）。
type ApproveConfig struct {
	Enable bool `yaml:"enable"`
}

// Load 从指定路径加载配置。
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}
	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config yaml: %w", err)
	}
	applyDefaults(cfg)
	if err := validate(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func applyDefaults(c *Config) {
	if c.Server.Listen == "" {
		c.Server.Listen = ":8080"
	}
	if len(c.Permission.NamespaceBlacklist) == 0 {
		c.Permission.NamespaceBlacklist = []string{"kube-system", "kube-public", "kube-node-lease"}
	}
	if len(c.Permission.MutationAllowedKinds) == 0 {
		c.Permission.MutationAllowedKinds = []string{"deployment"}
	}
	if c.Permission.ScaleLimit.MaxReplicas == 0 {
		c.Permission.ScaleLimit.MaxReplicas = 10
	}
	if c.Permission.LogLimits.MaxTailLines == 0 {
		c.Permission.LogLimits.MaxTailLines = 1000
	}
	if c.Permission.LogLimits.MaxBytes == 0 {
		c.Permission.LogLimits.MaxBytes = 5 << 20
	}
	if c.Permission.ListLimit <= 0 {
		c.Permission.ListLimit = 50
	}
	if c.Cache.ResyncHours <= 0 {
		c.Cache.ResyncHours = 2
	}
	if c.RateLimit.Enable && c.RateLimit.Burst <= 0 {
		c.RateLimit.Burst = int(c.RateLimit.QPS)
		if c.RateLimit.Burst < 1 {
			c.RateLimit.Burst = 1
		}
	}
	if c.Audit.Storage == "" {
		c.Audit.Storage = "log"
	}
}

func validate(c *Config) error {
	switch c.Audit.Storage {
	case "log", "redis":
	default:
		return fmt.Errorf("audit.storage must be \"log\" or \"redis\", got %q", c.Audit.Storage)
	}
	if c.Audit.Storage == "redis" && c.Audit.RedisAddr == "" {
		return fmt.Errorf("audit.redis_addr is required when audit.storage is redis")
	}
	return nil
}
