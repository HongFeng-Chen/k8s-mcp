package service

import (
	"strconv"
	"strings"
)

// 诊断知识库：把 k8s 常见错误/状态翻译成自然语言 + 建议动作。
// 纯领域处理，不调 apiserver，供 explain_error 工具使用。
type errorPattern struct {
	match       []string // 不区分大小写的关键字
	summary     string   // 一句话结论
	suggestions []string // 建议排查动作
}

var errorPatterns = []errorPattern{
	{
		match:   []string{"crashloopbackoff"},
		summary: "容器启动后反复崩溃退出（CrashLoopBackOff）。",
		suggestions: []string{
			"用 get_pod_logs 查看 crash 前的最后日志（可带 previous=true 看上一次崩溃日志）",
			"检查启动命令/参数是否正确、依赖服务是否可达",
			"检查环境变量与 ConfigMap/Secret 挂载是否齐全",
			"确认镜像 tag 是否正确，入口进程是否立刻退出",
		},
	},
	{
		match:   []string{"imagepullbackoff", "errimagepull"},
		summary: "镜像拉取失败（ImagePullBackOff）。",
		suggestions: []string{
			"确认镜像名与 tag 是否存在、是否拼写错误",
			"确认节点是否有权限访问该镜像仓库（私有仓库需 imagePullSecrets）",
			"检查 imagePullSecrets 是否配置、是否在目标命名空间存在",
			"注意：本 MCP Server 的镜像变更受白名单限制",
		},
	},
	{
		match:   []string{"oomkilled"},
		summary: "容器因内存超限被杀（OOMKilled）。",
		suggestions: []string{
			"确认应用真实内存用量，评估是否需要上调 memory limit",
			"检查是否存在内存泄漏（观察重启次数是否持续增长）",
			"调整 limit 需修改 Deployment 资源定义（本 Server 不提供该变更，建议走 GitOps）",
		},
	},
	{
		match:   []string{"evicted"},
		summary: "Pod 被驱逐（Evicted），通常是节点资源压力或节点不可用。",
		suggestions: []string{
			"用 get_events 查看驱逐原因（NodePressure / NodeNotReady 等）",
			"检查节点资源水位（CPU/内存/磁盘）",
			"若为系统性驱逐，考虑调整 requests/limits 或扩容节点",
		},
	},
	{
		match:   []string{"pending", "failedscheduling", "unschedulable"},
		summary: "Pod 处于 Pending，调度器无法分配节点。",
		suggestions: []string{
			"用 get_events 查看调度失败原因",
			"常见原因：集群资源不足、nodeSelector/affinity 不匹配、taint 未容忍、PVC 未绑定",
		},
	},
	{
		match:   []string{"forbidden", "cannot get resource", "is forbidden"},
		summary: "权限被拒绝（403 Forbidden）：当前 ServiceAccount 的 RBAC 权限不足。",
		suggestions: []string{
			"检查部署本 MCP Server 的 ServiceAccount 对应 Role/ClusterRole 是否包含该资源与动词",
			"遵循最小权限原则，按需扩权，而不是直接给 cluster-admin",
		},
	},
	{
		match:   []string{"not found", "404"},
		summary: "目标资源不存在（404）。",
		suggestions: []string{
			"先用 list 类工具确认命名空间与资源名拼写",
			"注意资源可能已被删除或属于其他命名空间",
		},
	},
	{
		match:   []string{"quota"},
		summary: "命名空间资源配额不足或已超限。",
		suggestions: []string{
			"用 kubectl describe resourcequota 查看（本 Server 未提供该工具）",
			"评估申请配额调整或优化资源 requests",
		},
	},
	{
		match:   []string{"readiness probe failed", "liveness probe failed"},
		summary: "健康探针失败，容器可能已启动但服务不可用。",
		suggestions: []string{
			"用 get_pod_logs 查看应用日志，确认服务是否真的 ready",
			"检查探针路径/端口/延迟时间是否与应用匹配",
		},
	},
	{
		match:   []string{"connection refused", "i/o timeout", "no such host", "dns"},
		summary: "网络/DNS 类错误：服务间调用失败。",
		suggestions: []string{
			"确认目标 Service 名称、端口、命名空间是否正确（同命名空间可短名）",
			"检查 CoreDNS 是否正常、NetworkPolicy 是否拦截",
			"用 list_services 确认目标服务存在及其 selector 是否匹配 Pod 标签",
		},
	},
	{
		match:       []string{"failed to pull", "manifest unknown"},
		summary:     "镜像 manifest 不存在：tag 写错或仓库中无此版本。",
		suggestions: []string{"核对镜像 tag；私有仓库确认认证信息有效。"},
	},
	{
		match:   []string{"termi", "killing", "deleting"},
		summary: "对象处于终止/删除流程中。",
		suggestions: []string{
			"若长时间 Terminating，检查 finalizer 是否卡住（需要人工介入，本 Server 不处理）",
		},
	},
}

// ExplainError 把 k8s 报错或 Pod 状态信息翻译成自然语言解释 + 排查建议。
func ExplainError(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return "未提供错误信息。请把 Pod 状态（如 CrashLoopBackOff）、事件或报错原文传入 error 参数。"
	}
	lower := strings.ToLower(text)

	var b strings.Builder
	matched := false
	for _, p := range errorPatterns {
		if !matchAny(lower, p.match) {
			continue
		}
		matched = true
		b.WriteString("结论：")
		b.WriteString(p.summary)
		b.WriteString("\n建议排查：\n")
		for i, s := range p.suggestions {
			b.WriteString("  ")
			b.WriteString(strconv.Itoa(i + 1))
			b.WriteString(". ")
			b.WriteString(s)
			b.WriteString("\n")
		}
		break // 命中最优先的一条即可
	}
	if !matched {
		b.WriteString("未能识别该错误的固定模式。\n")
		b.WriteString("通用建议：\n")
		b.WriteString("  1. 用 get_events 查看该对象的最近事件\n")
		b.WriteString("  2. 用 get_pod_logs 查看容器日志（崩溃时加 previous=true）\n")
		b.WriteString("  3. 用 get_pod 确认容器状态、重启次数与终止原因\n")
	}
	b.WriteString("\n原始输入：" + text)
	return b.String()
}

func matchAny(text string, keys []string) bool {
	for _, k := range keys {
		if strings.Contains(text, k) {
			return true
		}
	}
	return false
}
