// Package middleware 集中定义 HTTP 中间件。
//
// 顺序约定（见 router.Register 的注册顺序）：
//
//	RequestID → Recovery → Logger → CORS → 路由 → 组内限流/鉴权
//
// Recovery 保证任何 panic 都转成 500 而非断连；
// Logger 依赖 RequestID 做全链路关联。
package middleware

import (
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"go-backend/internal/core/apperr"
	"go-backend/internal/core/logger"
	"go-backend/internal/core/response"
)

// ContextKey 请求上下文中存放键的类型。
// 用自定义类型避免与第三方库的 string 键冲突。
type ContextKey string

// 上下文键名，handler 通过 GetUserID(c) 等辅助函数读取。
const (
	KeyRequestID ContextKey = "request_id"
	KeyUserID    ContextKey = "user_id"
	KeyUsername  ContextKey = "username"
	KeyRole      ContextKey = "role"
	KeyAPIKeyID  ContextKey = "api_key_id"
)

// ---- 上下文读取辅助函数 ----

// GetRequestID 取出链路 ID，不存在时返回空串。
func GetRequestID(c *gin.Context) string {
	v, _ := c.Get(string(KeyRequestID))
	s, _ := v.(string)
	return s
}

// GetUserID 取出当前登录用户 ID，未登录返回 0。
func GetUserID(c *gin.Context) uint {
	v, _ := c.Get(string(KeyUserID))
	id, _ := v.(uint)
	return id
}

// GetUsername 取出当前登录用户名。
func GetUsername(c *gin.Context) string {
	v, _ := c.Get(string(KeyUsername))
	s, _ := v.(string)
	return s
}

// GetRole 取出当前主体（用户或 API Key）的角色。
func GetRole(c *gin.Context) string {
	v, _ := c.Get(string(KeyRole))
	s, _ := v.(string)
	return s
}

// RequestID 为每个请求注入唯一 ID，用于日志串联。
// 上游（网关/前端）已传入 X-Request-ID 时沿用，保证跨服务链路完整。
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		rid := strings.TrimSpace(c.GetHeader("X-Request-ID"))
		// 上游传入的 ID 需限制长度，避免被用来注入超长日志。
		if rid == "" || len(rid) > 64 {
			rid = uuid.NewString()
		}
		c.Set(string(KeyRequestID), rid)
		c.Header("X-Request-ID", rid)
		c.Next()
	}
}

// Logger 记录每个请求的访问日志与耗时。
// 请求体不记录（可能含密码），只记录方法、路径、状态码、耗时与客户端 IP。
func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()
		rid := GetRequestID(c)

		fields := []interface{}{
			"rid", rid,
			"method", c.Request.Method,
			"path", path,
			"status", status,
			"latency", latency.String(),
			"ip", c.ClientIP(),
		}
		if query != "" {
			fields = append(fields, "query", query)
		}

		switch {
		case len(c.Errors) > 0:
			// gin 记录在 c.Errors 中的错误（含 panic 恢复信息）
			logger.Errorf("request %v err=%v", fields, c.Errors.String())
		case status >= 500:
			logger.Errorf("request %v", fields)
		case status >= 400:
			logger.Warnf("request %v", fields)
		default:
			logger.Infof("request %v", fields)
		}
	}
}

// CORS 处理跨域预检与放行。
// origins 含 ["*"] 时允许任意来源（开发环境）；
// 否则按白名单匹配，并且只回显命中的 Origin。
func CORS(origins []string) gin.HandlerFunc {
	allowAll := len(origins) == 0
	for _, o := range origins {
		if o == "*" {
			allowAll = true
			break
		}
	}
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin == "" {
			c.Next()
			return
		}

		allowed := allowAll
		if !allowAll {
			for _, o := range origins {
				if strings.EqualFold(o, origin) {
					allowed = true
					break
				}
			}
		}

		if allowed {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Vary", "Origin") // 缓存层需按 Origin 区分响应
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Request-ID, X-API-Key")
			c.Header("Access-Control-Expose-Headers", "X-Request-ID")
			c.Header("Access-Control-Max-Age", "600") // 预检结果缓存 10 分钟
		}

		// 预检请求直接短路，不再进入后续中间件与路由。
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// Recover 捕获 panic 并转成统一的 500 响应，防止连接被直接掐断。
// 堆栈只写日志，绝不返回给客户端。
func Recover() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				logger.Errorf("panic recovered rid=%s err=%v\n%s", GetRequestID(c), r, debug.Stack())

				if !c.Writer.Written() {
					c.Abort()
					response.Fail(c, apperr.Internal("服务器内部错误"))
					return
				}
				// 响应已部分写入，只能中止后续 handler。
				c.Abort()
			}
		}()
		c.Next()
	}
}

// Timeout 为请求设置整体超时，超时后返回 503。
// 注意：Gin 无法真正中断已在执行的 handler，仅能保证及时返回响应；
// 下游调用（数据库、外部 HTTP）仍需各自持有带超时的 context 才能被取消。
func Timeout(d time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		if d <= 0 {
			c.Next()
			return
		}
		timer := time.AfterFunc(d, func() {
			// 仅在响应尚未写入时接管，避免与正常返回竞争写状态码。
			if !c.Writer.Written() {
				c.AbortWithStatusJSON(http.StatusServiceUnavailable, response.Body{
					Code:    apperr.CodeUnavailable,
					Message: "请求处理超时",
					Data:    nil,
				})
			}
		})
		defer timer.Stop()
		c.Next()
	}
}
