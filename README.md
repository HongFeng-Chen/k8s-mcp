# k8s-mcp
1. MCP 只是 **命令翻译 + 安全校验层**：LLM 自然语言 → MCP Tool → 权限/白名单/风险校验 → client-go → kube-apiserver 2. 工具分三组：**查询组**（默认放行）、**变更组**（受控 + 审计）、**高危组**（默认关闭，v1 不实现 exec） 3. DDD 分层 + Helm 打包部署；不是 Operator、不是 APIService 4. 安全是第一优先级：RBAC 是兜底，业务安全校验全部收口在 MCP 应用/领域层
