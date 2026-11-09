package mcpadapter

import (
	"log/slog"
	"net/http"
	"strings"
	"sync"

	"golang.org/x/time/rate"

	"k8s-mcp/internal/infrastructure/observability"
	"k8s-mcp/pkg/shared"
)

// middleware 标准 HTTP 中间件。
type middleware func(http.Handler) http.Handler

// Chain 按顺序应用中间件（第一个最先执行）。
func Chain(h http.Handler, mws ...middleware) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}

// RecoverMiddleware 捕获 panic，防止单个请求打挂进程。
func RecoverMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				slog.Error("panic in mcp handler", "panic", rec, "path", r.URL.Path)
				http.Error(w, "internal server error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// HTTPMetrics 记录 HTTP 请求总数与状态码。
func HTTPMetrics(m *observability.Metrics) middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(sw, r)
			if m != nil {
				m.Inc("k8s_mcp_http_requests_total", map[string]string{
					"method": r.Method, "path": r.URL.Path, "code": itoa(sw.status),
				})
			}
		})
	}
}

type statusWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (w *statusWriter) WriteHeader(code int) {
	if !w.wroteHeader {
		w.status = code
		w.wroteHeader = true
	}
	w.ResponseWriter.WriteHeader(code)
}

// BearerAuth 接入层身份校验：
//   - 请求必须携带 Authorization: Bearer <token>；
//   - token 必须在配置的 auth_tokens 列表内；
//   - 校验通过后把身份写入 request context（审计用）。
func BearerAuth(authenticate func(token string) (string, bool)) middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := extractBearer(r.Header.Get("Authorization"))
			if token == "" {
				unauthorized(w, "missing bearer token")
				return
			}
			identity, ok := authenticate(token)
			if !ok {
				unauthorized(w, "invalid bearer token")
				return
			}
			next.ServeHTTP(w, r.WithContext(shared.WithIdentity(r.Context(), identity)))
		})
	}
}

func extractBearer(header string) string {
	const prefix = "Bearer "
	if len(header) <= len(prefix) || !strings.EqualFold(header[:len(prefix)], prefix) {
		return ""
	}
	return strings.TrimSpace(header[len(prefix):])
}

func unauthorized(w http.ResponseWriter, msg string) {
	w.Header().Set("WWW-Authenticate", `Bearer realm="k8s-mcp-server"`)
	http.Error(w, msg, http.StatusUnauthorized)
}

// RateLimiter 按身份限流：防止 LLM 循环疯狂调用打挂 apiserver。
type RateLimiter struct {
	mu       sync.Mutex
	qps      rate.Limit
	burst    int
	limiters map[string]*rate.Limiter
}

// NewRateLimiter 构建每身份限流器。
func NewRateLimiter(qps float64, burst int) *RateLimiter {
	return &RateLimiter{
		qps:      rate.Limit(qps),
		burst:    burst,
		limiters: make(map[string]*rate.Limiter),
	}
}

// Allow 某身份是否放行本次请求。
func (rl *RateLimiter) Allow(identity string) bool {
	if rl == nil {
		return true
	}
	rl.mu.Lock()
	l, ok := rl.limiters[identity]
	if !ok {
		l = rate.NewLimiter(rl.qps, rl.burst)
		rl.limiters[identity] = l
	}
	rl.mu.Unlock()
	return l.Allow()
}

// RateLimit 限流中间件（identity 已由 BearerAuth 写入 context）。
func RateLimit(rl *RateLimiter) middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !rl.Allow(shared.IdentityFrom(r.Context())) {
				w.Header().Set("Retry-After", "1")
				http.Error(w, "rate limit exceeded, slow down", http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
