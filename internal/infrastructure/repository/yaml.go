package repository

import (
	"gopkg.in/yaml.v3"
)

// yamlMarshal 通过 yaml.v3 输出（map 输入已脱敏）。
func yamlMarshal(v any) ([]byte, error) {
	return yaml.Marshal(v)
}
