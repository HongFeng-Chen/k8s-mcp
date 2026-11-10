# k8s-mcp

**Kubernetes MCP Server** —— 让 LLM 安全地看集群、有分寸地改集群。

MCP 只是**命令翻译 + 安全校验层**：LLM 自然语言 → MCP Tool → 权限 / 白名单 / 风险校验 → client-go → kube-apiserver。它不是 Operator，也不是 APIService；安全是第一优先级，RBAC 是兜底，业务安全校验全部收口在 MCP 应用 / 领域层。

## 设计原则

1. **查询组**工具默认放行；**变更组**受控 + 审计；**高危组**默认关闭（v1 不实现 exec）
2. DDD 分层（`application` / `domain` / `infrastructure` / `presentation`），Helm 打包部署
3. 全局只读开关 + 命名空间黑名单 + 仓库白名单 + 副本上下限，层层兜底

## 工具清单（13 个）

| 组 | 工具 |
|---|---|
| 诊断 | `explain_error`（纯本地推理，把 CrashLoopBackOff / OOMKilled 等翻译成排查建议） |
| 查询 | `list_pods` `list_deployments` `list_services` `list_namespaces` `get_pod` `get_deployment` `get_events` `get_pod_logs` `get_resource_yaml` |
| 变更（受控 + 审计） | `restart_deployment` `scale_deployment` `patch_deployment_image` |

查询默认走 Informer 本地缓存（`cache.enable`），降低 apiserver 压力；变更直连 apiserver。写能力由 `permission.read_only_mode` 控制：**为 true 时变更工具根本不注册**，LLM 看不到，领域层还会二次拦截。

## 配置

| 文件 | 用途 |
|---|---|
| `configs/config.yaml` | 生产 / 共享集群，默认**只读** |
| `configs/config.local.yaml` | 本地调试（kind / Docker Desktop），放开写能力，审计写 `logs/audit.jsonl` |

关键字段：`server.kubeconfig`（留空则 InCluster → `KUBECONFIG` → `~/.kube/config` 逐级回退）、`permission.namespace_blacklist`（kube-system 等变更禁入）、`permission.image_whitelist` / `scale_limit` / `log_limits`、`auth_tokens`（MCP 接入的 Bearer Token）。

## 本地运行

```bash
go build -o bin/k8s-mcp-server.exe ./cmd/server
./bin/k8s-mcp-server.exe -config configs/config.local.yaml
# healthz : http://127.0.0.1:8080/healthz
# MCP     : http://127.0.0.1:8080/mcp（StreamableHTTP，Bearer 鉴权）
```

### kind 集群连通性（重要）

kind 容器重建后，控制面 6443 通常**没有宿主机端口映射**，而 kubeconfig 里的旧地址（如 `https://127.0.0.1:58879`）随之失效——表象是 client-go reflector 持续报
`dial tcp 127.0.0.1:xxxxx: connectex: No connection could be made`。修复三步：

```bash
# 1. socat 边车打通宿主端口（已设 restart=unless-stopped，Docker 重启后自动恢复）
docker run -d --name k8s-api-proxy --network kind -p 6443:6443 \
  alpine/socat tcp-listen:6443,fork,reuseaddr tcp:desktop-control-plane:6443

# 2. kubeconfig 指向新端口 + 从控制面取真实 CA（Git Bash 写容器路径用 //etc/... 防转义）
kubectl config set-cluster desktop --server=https://127.0.0.1:6443 \
  --certificate-authority=/tmp/ca.crt --embed-certs=true

# 3. 集群重建过的话，管理员客户端证书也要重取（容器内 /etc/kubernetes/admin.conf）
```

更永久的方案：重建 kind 集群时在配置里加 `extraPortMappings`（宿主 6443 → 控制面 6443）。

## 接入 Meridians 平台

平台「MCP 服务与工具」页新增服务，URL 填 `http://127.0.0.1:8080/mcp`，鉴权头
`Authorization: Bearer <auth_tokens 中的 token>`。注意平台侧默认**禁止出网到内网 / 回环**，
本机 MCP 服务需要启动 go_agent 时设 `EGRESS_ALLOW_PRIVATE_HOSTS`（见 go_agent/README.md）。

接入后工具自动同步并向量化入库（Milvus `_tools`），Agent 即可通过语义召回调用。

## 部署（Helm）

```bash
helm install k8s-mcp deploy/charts/k8s-mcp
```

默认安装只读 ClusterRole（pod / deployment / service / configmap / event / namespace / node 的 get / list / watch）；需要变更能力时打开 `rbac.mutation.enabled`（仅 deployment 的 create / update / patch，不含 `pods/exec`），并把 `permission.readOnlyMode` 置为 false。容器内留空 `server.kubeconfig` 走 InClusterConfig。

## 可观测性

- `GET /metrics`：Prometheus 指标
- 审计：`audit.storage = log`（stdout 或文件）或 `redis`
- 限流：按身份 QPS / Burst（`rate_limit.qps=5, burst=10`）
