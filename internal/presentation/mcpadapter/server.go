// Package mcpadapter MCP 协议适配层：注册工具、挂载 StreamableHTTP 传输。
// 本层只做协议转换，无业务逻辑，禁止直接调用 client-go。
package mcpadapter

import (
	"net/http"

	"github.com/google/wire"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"k8s-mcp/internal/application"
	"k8s-mcp/internal/infrastructure/observability"
	"k8s-mcp/internal/presentation/mcpadapter/tools"
)

// ProviderSet MCP 适配层 Provider 集合（wire）。
// RateLimiter 参数来自 config，由 cmd/server 的 provideRateLimiter 提供。
var ProviderSet = wire.NewSet(NewMCPServerAdapter)

// MCPServerAdapter MCP 服务端适配器。
type MCPServerAdapter struct {
	mcpServer *mcp.Server
}

// NewMCPServerAdapter 构建 MCP Server 并按权限模式注册工具（wire Provider）。
func NewMCPServerAdapter(
	podUC *application.PodUsecase,
	depUC *application.DeploymentUsecase,
	clusterUC *application.ClusterUsecase,
	diagUC *application.DiagnosticsUsecase,
	metrics *observability.Metrics,
	readOnlyMode bool, // 由 cmd/server 的适配 Provider 从 config 提供
) *MCPServerAdapter {
	srv := mcp.NewServer(&mcp.Implementation{
		Name:    "k8s-mcp-server",
		Version: "0.1.0",
	}, nil)

	m := metrics

	// ---- 查询组（默认开启，无审批） ----
	tools.Register(srv, m, tools.NewListNamespacesTool(clusterUC))
	tools.Register(srv, m, tools.NewListPodsTool(podUC))
	tools.Register(srv, m, tools.NewGetPodTool(podUC))
	tools.Register(srv, m, tools.NewGetPodLogsTool(podUC))
	tools.Register(srv, m, tools.NewListDeploymentsTool(depUC))
	tools.Register(srv, m, tools.NewGetDeploymentTool(depUC))
	tools.Register(srv, m, tools.NewListServicesTool(clusterUC))
	tools.Register(srv, m, tools.NewGetEventsTool(clusterUC))
	tools.Register(srv, m, tools.NewGetResourceYAMLTool(podUC, depUC, clusterUC))
	tools.Register(srv, m, tools.NewExplainErrorTool(diagUC))

	// ---- 变更组（read_only_mode=true 时完全不注册，LLM 看不到写能力；
	//      即使配置被改，领域层 RiskControlService 仍会二次拦截） ----
	if !readOnlyMode {
		tools.Register(srv, m, tools.NewRestartDeploymentTool(depUC))
		tools.Register(srv, m, tools.NewScaleDeploymentTool(depUC))
		tools.Register(srv, m, tools.NewPatchDeploymentImageTool(depUC))
	}

	// ---- 高危组（exec/delete/create）：v1 不实现，不注册。
	//      exec 风险极高（容器逃逸、读宿主机），未来如需开放必须走审批流。 ----

	return &MCPServerAdapter{mcpServer: srv}
}

// Handler 返回挂好中间件的 /mcp 端点。
func (a *MCPServerAdapter) Handler(deps HandlerDeps) http.Handler {
	streamable := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return a.mcpServer
	}, nil)

	chain := Chain(streamable,
		RecoverMiddleware,
		HTTPMetrics(deps.Metrics),
		BearerAuth(deps.Authenticate),
		RateLimit(deps.RateLimiter),
	)
	return chain
}

// HandlerDeps HTTP 中间件依赖。
type HandlerDeps struct {
	Metrics      *observability.Metrics
	Authenticate func(token string) (string, bool) // token -> (identity, ok)
	RateLimiter  *RateLimiter
}
