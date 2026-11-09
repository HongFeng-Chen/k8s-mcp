// Package repository 实现 internal/domain/repository 定义的仓储接口，
// 负责 client-go 对象 -> 领域实体的映射；查询优先走 informer 缓存。
package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	"k8s-mcp/internal/domain/entity"
	domainerrors "k8s-mcp/internal/domain/errors"
	"k8s-mcp/internal/domain/vo"
	"k8s-mcp/internal/infrastructure/k8sclient"
	"k8s-mcp/pkg/shared"
)

// PodRepo Pod 仓储实现。
type PodRepo struct {
	client *k8sclient.Client
}

func NewPodRepo(client *k8sclient.Client) *PodRepo {
	return &PodRepo{client: client}
}

// List 列出 Pod：缓存开启且无字段选择器时走 lister，否则直连。
func (r *PodRepo) List(ctx context.Context, ns, labelSelector string, limit int) ([]entity.Pod, error) {
	sel, err := labels.Parse(labelSelector)
	if err != nil {
		return nil, fmt.Errorf("%w: bad label selector: %v", domainerrors.ErrInvalidArgument, err)
	}

	if r.client.HasCache() {
		// informer 缓存路径：标签选择器由 lister 内存过滤（limit 仍生效）
		var pods []*corev1.Pod
		if ns == "" {
			pods, err = r.client.Cache().Pods().List(sel)
		} else {
			pods, err = r.client.Cache().Pods().Pods(ns).List(sel)
		}
		if err != nil {
			return nil, fmt.Errorf("list pods from cache: %w", err)
		}
		out := make([]entity.Pod, 0, len(pods))
		for _, p := range pods {
			out = append(out, toDomainPod(p))
		}
		return truncatePods(out, limit), nil
	}

	listOpts := metav1.ListOptions{LabelSelector: labelSelector}
	if limit > 0 {
		listOpts.Limit = int64(limit)
	}
	list, err := r.client.Clientset.CoreV1().Pods(ns).List(ctx, listOpts)
	if err != nil {
		return nil, fmt.Errorf("list pods: %w", err)
	}
	out := make([]entity.Pod, 0, len(list.Items))
	for i := range list.Items {
		out = append(out, toDomainPod(&list.Items[i]))
	}
	return out, nil
}

// Get 获取单个 Pod。
func (r *PodRepo) Get(ctx context.Context, ns, name string) (*entity.Pod, error) {
	if r.client.HasCache() {
		p, err := r.client.Cache().Pods().Pods(ns).Get(name)
		if err == nil {
			d := toDomainPod(p)
			return &d, nil
		}
		// 缓存未命中（可能尚未同步）时回退直连
	}
	p, err := r.client.Clientset.CoreV1().Pods(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, mapK8sGetErr(err, "pod", ns, name)
	}
	d := toDomainPod(p)
	return &d, nil
}

// GetLogs 获取容器日志。
func (r *PodRepo) GetLogs(ctx context.Context, ns, name string, q vo.LogQuery) (*entity.PodLog, error) {
	opts := &corev1.PodLogOptions{
		Container:  q.Container,
		TailLines:  &q.TailLines,
		LimitBytes: &q.LimitBytes,
		Previous:   q.Previous,
	}
	raw, err := r.client.Clientset.CoreV1().Pods(ns).GetLogs(name, opts).DoRaw(ctx)
	if err != nil {
		return nil, fmt.Errorf("get pod logs: %w", err)
	}
	text := string(raw)
	truncated := len(raw) >= int(q.LimitBytes)
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	return &entity.PodLog{Namespace: ns, PodName: name, Lines: lines, Truncated: truncated}, nil
}

// GetYAML 获取 Pod YAML（脱敏）。
func (r *PodRepo) GetYAML(ctx context.Context, ns, name string) (string, error) {
	p, err := r.client.Clientset.CoreV1().Pods(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return "", mapK8sGetErr(err, "pod", ns, name)
	}
	return toRedactedYAML(p)
}

// toRedactedYAML 对象 -> JSON -> map（脱敏）-> YAML。
func toRedactedYAML(obj any) (string, error) {
	data, err := json.Marshal(obj)
	if err != nil {
		return "", fmt.Errorf("marshal object: %w", err)
	}
	var generic any
	if err := json.Unmarshal(data, &generic); err != nil {
		return "", fmt.Errorf("unmarshal object: %w", err)
	}
	shared.RedactMap(generic)
	y, err := yamlMarshal(generic)
	if err != nil {
		return "", err
	}
	return string(y), nil
}

func mapK8sGetErr(err error, kind, ns, name string) error {
	if apierrors.IsNotFound(err) {
		return fmt.Errorf("%w: %s %s/%s", domainerrors.ErrNotFound, kind, ns, name)
	}
	return fmt.Errorf("get %s %s/%s: %w", kind, ns, name, err)
}

// --- 映射函数 ---

func toDomainPod(p *corev1.Pod) entity.Pod {
	out := entity.Pod{
		Namespace: p.Namespace,
		Name:      p.Name,
		Phase:     string(p.Status.Phase),
		NodeName:  p.Spec.NodeName,
		PodIP:     p.Status.PodIP,
		Labels:    p.Labels,
		CreatedAt: p.CreationTimestamp.Time,
		Reason:    p.Status.Reason,
	}
	if p.DeletionTimestamp != nil {
		out.DeleteRequested = true
	}
	for _, cs := range p.Status.ContainerStatuses {
		out.RestartCount += cs.RestartCount
		out.TotalContainers++
		if cs.Ready {
			out.ReadyContainers++
		}
		if cs.State.Terminated != nil && cs.State.Terminated.Reason == "OOMKilled" {
			out.OOMKilled = true
		}
	}
	return out
}

func truncatePods(in []entity.Pod, limit int) []entity.Pod {
	if limit <= 0 || len(in) <= limit {
		return in
	}
	return in[:limit]
}
