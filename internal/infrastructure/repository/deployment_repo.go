package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"

	"k8s-mcp/internal/domain/entity"
	domainerrors "k8s-mcp/internal/domain/errors"
	"k8s-mcp/internal/domain/vo"
	"k8s-mcp/internal/infrastructure/k8sclient"
)

// restartedAtAnnotation 滚动重启注解（与 kubectl rollout restart 一致）。
const restartedAtAnnotation = "kubectl.kubernetes.io/restartedAt"

// DeploymentRepo Deployment 仓储实现。
type DeploymentRepo struct {
	client *k8sclient.Client
}

func NewDeploymentRepo(client *k8sclient.Client) *DeploymentRepo {
	return &DeploymentRepo{client: client}
}

func (r *DeploymentRepo) List(ctx context.Context, ns, labelSelector string, limit int) ([]entity.Deployment, error) {
	sel, err := labels.Parse(labelSelector)
	if err != nil {
		return nil, fmt.Errorf("%w: bad label selector: %v", domainerrors.ErrInvalidArgument, err)
	}

	// informer 缓存路径：标签选择器由 lister 内存过滤
	if r.client.HasCache() {
		var deps []*appsv1.Deployment
		if ns == "" {
			deps, err = r.client.Cache().Deployments().List(sel)
		} else {
			deps, err = r.client.Cache().Deployments().Deployments(ns).List(sel)
		}
		if err != nil {
			return nil, fmt.Errorf("list deployments from cache: %w", err)
		}
		out := make([]entity.Deployment, 0, len(deps))
		for _, d := range deps {
			out = append(out, toDomainDeployment(d))
		}
		return truncateDeployments(out, limit), nil
	}

	listOpts := metav1.ListOptions{LabelSelector: labelSelector}
	if limit > 0 {
		listOpts.Limit = int64(limit)
	}
	list, err := r.client.Clientset.AppsV1().Deployments(ns).List(ctx, listOpts)
	if err != nil {
		return nil, fmt.Errorf("list deployments: %w", err)
	}
	out := make([]entity.Deployment, 0, len(list.Items))
	for i := range list.Items {
		out = append(out, toDomainDeployment(&list.Items[i]))
	}
	return out, nil
}

func (r *DeploymentRepo) Get(ctx context.Context, ns, name string) (*entity.Deployment, error) {
	// 缓存命中直接返回；未命中（可能尚未同步）回退直连
	if r.client.HasCache() {
		if d, err := r.client.Cache().Deployments().Deployments(ns).Get(name); err == nil {
			out := toDomainDeployment(d)
			return &out, nil
		}
	}
	d, err := r.client.Clientset.AppsV1().Deployments(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, mapK8sGetErr(err, "deployment", ns, name)
	}
	out := toDomainDeployment(d)
	return &out, nil
}

func (r *DeploymentRepo) GetYAML(ctx context.Context, ns, name string) (string, error) {
	d, err := r.client.Clientset.AppsV1().Deployments(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return "", mapK8sGetErr(err, "deployment", ns, name)
	}
	return toRedactedYAML(d)
}

// Restart 打 restartedAt 注解触发滚动重启（k8s 原生安全方式，不走 delete）。
func (r *DeploymentRepo) Restart(ctx context.Context, ns, name string) error {
	patch := map[string]any{
		"spec": map[string]any{
			"template": map[string]any{
				"metadata": map[string]any{
					"annotations": map[string]any{
						restartedAtAnnotation: timeNowRFC3339(),
					},
				},
			},
		},
	}
	data, err := json.Marshal(patch)
	if err != nil {
		return fmt.Errorf("marshal restart patch: %w", err)
	}
	_, err = r.client.Clientset.AppsV1().Deployments(ns).Patch(ctx, name, types.StrategicMergePatchType, data, metav1.PatchOptions{})
	if err != nil {
		return fmt.Errorf("restart deployment %s/%s: %w", ns, name, err)
	}
	return nil
}

// Scale 副本扩缩（strategic merge patch spec.replicas，原子操作）。
func (r *DeploymentRepo) Scale(ctx context.Context, ns, name string, replicas vo.ReplicaCount) error {
	data, err := json.Marshal(map[string]any{
		"spec": map[string]any{"replicas": replicas.Int32()},
	})
	if err != nil {
		return fmt.Errorf("marshal scale patch: %w", err)
	}
	_, err = r.client.Clientset.AppsV1().Deployments(ns).Patch(ctx, name, types.StrategicMergePatchType, data, metav1.PatchOptions{})
	if err != nil {
		return fmt.Errorf("scale deployment %s/%s: %w", ns, name, err)
	}
	return nil
}

// PatchImage 更新指定容器镜像（strategic merge 按 container name 合并）。
func (r *DeploymentRepo) PatchImage(ctx context.Context, ns, name, container string, image vo.ImageRef) error {
	if container == "" {
		return fmt.Errorf("%w: container name is required", domainerrors.ErrInvalidArgument)
	}
	data, err := json.Marshal(map[string]any{
		"spec": map[string]any{
			"template": map[string]any{
				"spec": map[string]any{
					"containers": []any{
						map[string]any{"name": container, "image": image.String()},
					},
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("marshal image patch: %w", err)
	}
	_, err = r.client.Clientset.AppsV1().Deployments(ns).Patch(ctx, name, types.StrategicMergePatchType, data, metav1.PatchOptions{})
	if err != nil {
		return fmt.Errorf("patch deployment image %s/%s: %w", ns, name, err)
	}
	return nil
}

func toDomainDeployment(d *appsv1.Deployment) entity.Deployment {
	out := entity.Deployment{
		Namespace:         d.Namespace,
		Name:              d.Name,
		Replicas:          derefInt32(d.Spec.Replicas),
		ReadyReplicas:     d.Status.ReadyReplicas,
		AvailableReplicas: d.Status.AvailableReplicas,
		UpdatedReplicas:   d.Status.UpdatedReplicas,
		Generation:        d.Generation,
		CreatedAt:         d.CreationTimestamp.Time,
	}
	for _, c := range d.Spec.Template.Spec.Containers {
		out.Images = append(out.Images, c.Image)
	}
	return out
}

func derefInt32(p *int32) int32 {
	if p == nil {
		return 0
	}
	return *p
}

// truncateDeployments 按单次返回上限截断（缓存路径在客户端做 limit）。
func truncateDeployments(in []entity.Deployment, limit int) []entity.Deployment {
	if limit <= 0 || len(in) <= limit {
		return in
	}
	return in[:limit]
}

// timeNowRFC3339 集中取时（便于测试时替换）。
var timeNowRFC3339 = func() string {
	return time.Now().Format(time.RFC3339)
}
