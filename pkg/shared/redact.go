// Package shared 提供跨层通用的小工具。
package shared

import (
	"strings"
)

// sensitiveEnvKeys 环境变量名中疑似敏感的关键字。
var sensitiveEnvKeys = []string{"password", "passwd", "secret", "token", "api_key", "apikey", "private", "credential", "auth"}

// RedactMap 对反序列化后的 k8s 对象做脱敏：
//   - Secret 的 data/stringData 值整体替换；
//   - env 中带敏感关键字的 value 替换；
//   - 保留键名，便于 LLM 理解结构。
func RedactMap(v any) {
	switch t := v.(type) {
	case map[string]any:
		redactMap(t)
	case []any:
		for _, item := range t {
			RedactMap(item)
		}
	}
}

func redactMap(m map[string]any) {
	for k, v := range m {
		lk := strings.ToLower(k)
		switch {
		case lk == "data" || lk == "stringdata":
			if sub, ok := v.(map[string]any); ok {
				for sk := range sub {
					sub[sk] = "***REDACTED***"
				}
			}
			continue
		case lk == "env":
			if list, ok := v.([]any); ok {
				redactEnvList(list)
			}
			continue
		case lk == "valuefrom":
			continue // 引用型 env 不含明文
		}
		RedactMap(v)
	}
}

func redactEnvList(list []any) {
	for _, item := range list {
		env, ok := item.(map[string]any)
		if !ok {
			continue
		}
		name, _ := env["name"].(string)
		if isSensitiveEnvName(name) {
			if _, hasValue := env["value"]; hasValue {
				env["value"] = "***REDACTED***"
			}
		}
	}
}

func isSensitiveEnvName(name string) bool {
	ln := strings.ToLower(name)
	for _, key := range sensitiveEnvKeys {
		if strings.Contains(ln, key) {
			return true
		}
	}
	return false
}
