package entity

import "time"

// Service Service 领域实体。
type Service struct {
	Namespace   string
	Name        string
	Type        string // ClusterIP / NodePort / LoadBalancer / ExternalName
	ClusterIP   string
	ExternalIPs []string
	Ports       []ServicePort
	Selector    map[string]string
	CreatedAt   time.Time
}

// ServicePort 服务端口映射。
type ServicePort struct {
	Name       string
	Port       int32
	TargetPort string
	NodePort   int32
	Protocol   string
}

// SelectorString 形如 "app=nginx,tier=web" 的选择器摘要。
func (s Service) SelectorString() string {
	out := ""
	for k, v := range s.Selector {
		if out != "" {
			out += ","
		}
		out += k + "=" + v
	}
	return out
}
