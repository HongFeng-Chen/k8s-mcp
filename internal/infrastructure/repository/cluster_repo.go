package repository

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s-mcp/internal/domain/entity"
	"k8s-mcp/internal/infrastructure/k8sclient"
)

// ClusterRepo Namespace / Service / Event 仓储实现。
type ClusterRepo struct {
	client *k8sclient.Client
}

func NewClusterRepo(client *k8sclient.Client) *ClusterRepo {
	return &ClusterRepo{client: client}
}

func (r *ClusterRepo) ListNamespaces(ctx context.Context) ([]entity.NamespaceInfo, error) {
	list, err := r.client.Clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list namespaces: %w", err)
	}
	out := make([]entity.NamespaceInfo, 0, len(list.Items))
	for i := range list.Items {
		ns := &list.Items[i]
		out = append(out, entity.NamespaceInfo{
			Name:   ns.Name,
			Status: string(ns.Status.Phase),
			Labels: ns.Labels,
		})
	}
	return out, nil
}

func (r *ClusterRepo) ListServices(ctx context.Context, ns string, limit int) ([]entity.Service, error) {
	opts := metav1.ListOptions{}
	if limit > 0 {
		opts.Limit = int64(limit)
	}
	list, err := r.client.Clientset.CoreV1().Services(ns).List(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("list services: %w", err)
	}
	out := make([]entity.Service, 0, len(list.Items))
	for i := range list.Items {
		out = append(out, toDomainService(&list.Items[i]))
	}
	return out, nil
}

func (r *ClusterRepo) GetService(ctx context.Context, ns, name string) (*entity.Service, error) {
	svc, err := r.client.Clientset.CoreV1().Services(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, mapK8sGetErr(err, "service", ns, name)
	}
	out := toDomainService(svc)
	return &out, nil
}

func (r *ClusterRepo) GetServiceYAML(ctx context.Context, ns, name string) (string, error) {
	svc, err := r.client.Clientset.CoreV1().Services(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return "", mapK8sGetErr(err, "service", ns, name)
	}
	return toRedactedYAML(svc)
}

// ListEvents 按 ns + 对象名过滤事件。
func (r *ClusterRepo) ListEvents(ctx context.Context, ns, objectName string, limit int) ([]entity.Event, error) {
	opts := metav1.ListOptions{}
	if objectName != "" {
		// 字段选择器精确过滤到某个对象
		opts.FieldSelector = "involvedObject.name=" + objectName
		if ns != "" {
			opts.FieldSelector += ",involvedObject.namespace=" + ns
		}
	}
	if limit > 0 {
		opts.Limit = int64(limit)
	}
	list, err := r.client.Clientset.CoreV1().Events(ns).List(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("list events: %w", err)
	}
	out := make([]entity.Event, 0, len(list.Items))
	for i := range list.Items {
		out = append(out, toDomainEvent(&list.Items[i]))
	}
	return out, nil
}

// --- 映射函数 ---

func toDomainService(svc *corev1.Service) entity.Service {
	out := entity.Service{
		Namespace:   svc.Namespace,
		Name:        svc.Name,
		Type:        string(svc.Spec.Type),
		ClusterIP:   svc.Spec.ClusterIP,
		ExternalIPs: append([]string(nil), svc.Spec.ExternalIPs...),
		Selector:    svc.Spec.Selector,
		CreatedAt:   svc.CreationTimestamp.Time,
	}
	for _, p := range svc.Spec.Ports {
		out.Ports = append(out.Ports, entity.ServicePort{
			Name:       p.Name,
			Port:       p.Port,
			TargetPort: p.TargetPort.String(),
			NodePort:   p.NodePort,
			Protocol:   string(p.Protocol),
		})
	}
	return out
}

func toDomainEvent(e *corev1.Event) entity.Event {
	return entity.Event{
		Namespace:     e.Namespace,
		Type:          e.Type,
		Reason:        e.Reason,
		ObjectKind:    e.InvolvedObject.Kind,
		ObjectName:    e.InvolvedObject.Name,
		Message:       e.Message,
		Count:         e.Count,
		FirstSeen:     e.FirstTimestamp.Time,
		LastSeen:      e.LastTimestamp.Time,
		SourceContext: e.Source.Component,
	}
}
