// Package k8sclient 封装 client-go：rest config 构建、clientset、Informer 缓存。
// 查询类走本地缓存降低 apiserver 压力；变更类直接打 apiserver。
package k8sclient

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"k8s-mcp/internal/infrastructure/config"
)

// Client 聚合 clientset 与可选的 informer 缓存。
// 作为 wire 的 Provider：构造即完成缓存预热，Close 停止 informer。
type Client struct {
	Clientset *kubernetes.Clientset
	cache     *Cache
	stopCh    chan struct{}
}

// NewClient 构建客户端（wire Provider）：
//   - rest config：显式 kubeconfig > InClusterConfig > 本地默认 kubeconfig（KUBECONFIG 或 ~/.kube/config）；
//   - cfg.Cache.Enable=true 时启动 informer 并等待首次同步。
func NewClient(cfg *config.Config) (*Client, error) {
	restCfg, err := buildRestConfig(cfg.Server.Kubeconfig)
	if err != nil {
		return nil, err
	}

	// MCP 场景 QPS 有限且走缓存，适当放宽客户端限流，避免诊断高峰期被客户端限流拖慢
	restCfg.QPS = 20
	restCfg.Burst = 40

	cs, err := kubernetes.NewForConfig(restCfg)
	if err != nil {
		return nil, fmt.Errorf("create clientset: %w", err)
	}
	client := &Client{Clientset: cs}

	if cfg.Cache.Enable {
		client.stopCh = make(chan struct{})
		if err := client.EnableCache(client.stopCh); err != nil {
			return nil, fmt.Errorf("warm up informer cache: %w", err)
		}
	}
	return client, nil
}

// buildRestConfig 按优先级构建 rest.Config：
//  1. 配置里显式指定 kubeconfig（支持 ~ 展开）；
//  2. 集群内 InClusterConfig；
//  3. 本地默认 kubeconfig（KUBECONFIG 环境变量，否则 ~/.kube/config）。
func buildRestConfig(kubeconfigPath string) (*rest.Config, error) {
	if kubeconfigPath != "" {
		path, err := expandHome(kubeconfigPath)
		if err != nil {
			return nil, err
		}
		c, err := clientcmd.BuildConfigFromFlags("", path)
		if err != nil {
			return nil, fmt.Errorf("build rest config from kubeconfig %q: %w", path, err)
		}
		return c, nil
	}

	inClusterCfg, inErr := rest.InClusterConfig()
	if inErr == nil {
		return inClusterCfg, nil
	}

	// 注意：必须用 NewDefaultClientConfigLoadingRules()。
	// 直接构造 &clientcmd.ClientConfigLoadingRules{} 没有 Precedence，
	// 不会读取任何文件，会以 "no configuration has been provided" 失败。
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	localCfg, localErr := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		rules, &clientcmd.ConfigOverrides{},
	).ClientConfig()
	if localErr == nil {
		return localCfg, nil
	}

	return nil, fmt.Errorf("no usable kubeconfig: in-cluster failed (%v); "+
		"local kubeconfig failed (%w); "+
		"本地调试请在配置文件里设置 server.kubeconfig（如 \"~/.kube/config\"），"+
		"或直接使用 -config configs/config.local.yaml", inErr, localErr)
}

// expandHome 展开路径首部的 ~（Windows 下 os.UserHomeDir 即 %USERPROFILE%）。
func expandHome(p string) (string, error) {
	if p != "~" && !strings.HasPrefix(p, "~/") && !strings.HasPrefix(p, `~\`) {
		return p, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir for %q: %w", p, err)
	}
	rest := strings.TrimPrefix(strings.TrimPrefix(strings.TrimPrefix(p, "~"), "/"), `\`)
	if rest == "" {
		return home, nil
	}
	return filepath.Join(home, rest), nil
}

// EnableCache 开启 informer 缓存并等待首次同步完成。
func (c *Client) EnableCache(stopCh <-chan struct{}) error {
	resync := time.Duration(2) * time.Hour
	factory := informers.NewSharedInformerFactory(c.Clientset, resync)
	cache := NewCache(factory)
	if err := cache.Sync(stopCh); err != nil {
		return err
	}
	c.cache = cache
	return nil
}

// Cache 返回缓存实例（未开启时为 nil，调用方自行回退直连）。
func (c *Client) Cache() *Cache { return c.cache }

// HasCache 是否已开启缓存。
func (c *Client) HasCache() bool { return c.cache != nil }

// Close 停止 informer（优雅退出时调用）。
func (c *Client) Close() {
	if c.stopCh != nil {
		select {
		case <-c.stopCh: // 已关闭
		default:
			close(c.stopCh)
		}
	}
}
