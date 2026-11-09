// Package observability 极简 Prometheus 指标注册中心。
// 不引入 prometheus/client_golang（保持依赖极简），手工输出 text exposition 格式：
//
//	# HELP k8s_mcp_tool_calls_total tool calls
//	# TYPE k8s_mcp_tool_calls_total counter
//	k8s_mcp_tool_calls_total{tool="list_pods",result="ok"} 12
package observability

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
)

// Metrics 线程安全的标签化计数器集合。
type Metrics struct {
	mu       sync.Mutex
	counters map[string]float64
	labels   map[string][]string // series key -> 排序后的 label 名
}

func NewMetrics() *Metrics {
	return &Metrics{
		counters: make(map[string]float64),
		labels:   make(map[string][]string),
	}
}

// Inc 递增一个带标签的计数器（标签键必须固定，避免基数爆炸）。
func (m *Metrics) Inc(name string, labels map[string]string) {
	key, lv := seriesKey(name, labels)
	m.mu.Lock()
	m.labels[key] = lv
	m.counters[key]++
	m.mu.Unlock()
}

// Handler 输出 Prometheus text 格式。
func (m *Metrics) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		m.mu.Lock()
		defer m.mu.Unlock()

		// 按指标名分组输出
		byName := make(map[string][]string)
		for key := range m.counters {
			name := key
			if i := strings.Index(key, "{"); i >= 0 {
				name = key[:i]
			}
			byName[name] = append(byName[name], key)
		}
		names := make([]string, 0, len(byName))
		for n := range byName {
			names = append(names, n)
		}
		sort.Strings(names)

		var b strings.Builder
		for _, name := range names {
			keys := byName[name]
			sort.Strings(keys)
			fmt.Fprintf(&b, "# HELP %s %s\n# TYPE %s counter\n", name, name, name)
			for _, k := range keys {
				fmt.Fprintf(&b, "%s %d\n", k, int64(m.counters[k]))
			}
		}
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		_, _ = w.Write([]byte(b.String()))
	})
}

func seriesKey(name string, labels map[string]string) (string, []string) {
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	b.WriteString(name)
	b.WriteByte('{')
	for i, k := range keys {
		if i > 0 {
			b.WriteByte(',')
		}
		fmt.Fprintf(&b, "%s=%q", k, labels[k])
	}
	b.WriteByte('}')
	return b.String(), keys
}
