package k8sclient

import (
	"fmt"

	informers "k8s.io/client-go/informers"
	appsv1 "k8s.io/client-go/informers/apps/v1"
	corev1 "k8s.io/client-go/informers/core/v1"
	listersappsv1 "k8s.io/client-go/listers/apps/v1"
	listerscorev1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
)

// Cache 基于 SharedInformerFactory 的本地缓存。
// 只缓存 Pod 与 Deployment（诊断最高频的两类查询）；
// Event / Service / 日志 / YAML 直连 apiserver。
type Cache struct {
	factory        informers.SharedInformerFactory
	podInformer    corev1.PodInformer
	deployInformer appsv1.DeploymentInformer
}

func NewCache(factory informers.SharedInformerFactory) *Cache {
	podInformer := factory.Core().V1().Pods()
	deployInformer := factory.Apps().V1().Deployments()

	// 关键：必须显式构造 Informer，才能注册进 factory 的 informers map。
	// lister 是懒注册的（Lister() 内部才调 Informer()），若等到首次查询才注册，
	// factory.Start() 已经执行过，这些 informer 永远不会被启动，缓存会一直为空。
	_ = podInformer.Informer()
	_ = deployInformer.Informer()

	return &Cache{
		factory:        factory,
		podInformer:    podInformer,
		deployInformer: deployInformer,
	}
}

// Sync 启动 informer 并等待缓存首次同步（显式按 HasSynced 等待，同步失败直接报错）。
func (c *Cache) Sync(stopCh <-chan struct{}) error {
	c.factory.Start(stopCh)
	if !cache.WaitForCacheSync(stopCh,
		c.podInformer.Informer().HasSynced,
		c.deployInformer.Informer().HasSynced,
	) {
		return fmt.Errorf("informer cache failed to sync (pod/deployment)")
	}
	return nil
}

// Pods 返回 Pod Lister。
func (c *Cache) Pods() listerscorev1.PodLister { return c.podInformer.Lister() }

// Deployments 返回 Deployment Lister。
func (c *Cache) Deployments() listersappsv1.DeploymentLister { return c.deployInformer.Lister() }
