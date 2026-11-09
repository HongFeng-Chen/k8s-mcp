package audit

import "encoding/json"

// jsonMarshal 独立封装，避免与响应编码逻辑耦合。
func jsonMarshal(v any) (string, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
