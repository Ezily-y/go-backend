// 鉴权中间件：JWT 与 API Key 两种身份载体。
//
// 两种载体写入相同的上下文键（user_id / role），
// 业务层无需关心请求是人（浏览器）还是机器（工具）发起的。
package middleware

import (
	"context"
	"strings"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"

	"go-backend/internal/apperr"
	"go-backend/internal/auth"
	"go-backend/internal/response"
)

// ---- JWT 认证 ----

// JWT 校验 Authorization: Bearer <token>，通过后把身份写入上下文。
func JWT(svc *auth.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw, err := extractBearer(c)
		if err != nil {
			abort(c, apperr.Unauthorized("缺少访问令牌"))
			return
		}

		claims, err := svc.ParseWithExpectation(raw, auth.TokenAccess)
		if err != nil {
			abort(c, apperr.Unauthorized("访问令牌无效或已过期"))
			return
		}

		c.Set(string(KeyUserID), claims.UserID)
		c.Set(string(KeyUsername), claims.Username)
		c.Set(string(KeyRole), string(claims.Role))
		c.Next()
	}
}

// extractBearer 从 Authorization 头提取 Bearer 令牌。
func extractBearer(c *gin.Context) (string, error) {
	h := c.GetHeader("Authorization")
	if h == "" {
		return "", apperr.New(apperr.CodeUnauthorized, "缺少 Authorization 头")
	}
	parts := strings.SplitN(h, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", apperr.New(apperr.CodeUnauthorized, "Authorization 格式应为 Bearer <token>")
	}
	token := strings.TrimSpace(parts[1])
	if token == "" {
		return "", apperr.New(apperr.CodeUnauthorized, "令牌为空")
	}
	return token, nil
}

// ---- RBAC ----

// RequireRole 要求当前主体具备指定角色之一（任一命中即放行）。
// 角色优先级：admin > editor > viewer，admin 恒通过。
func RequireRole(roles ...string) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(roles))
	for _, r := range roles {
		allowed[r] = struct{}{}
	}
	return func(c *gin.Context) {
		role := GetRole(c)

		if _, ok := allowed[role]; ok {
			c.Next()
			return
		}
		// admin 是超级管理员，永远有权访问受限路由。
		if role == string(roleAdmin) {
			c.Next()
			return
		}
		abort(c, apperr.Forbidden("权限不足，需要角色: "+strings.Join(roles, "/")))
	}
}

// roleAdmin 常量，避免在多处硬编码字符串。
const roleAdmin = "admin"

// RequireAdmin 是 RequireRole 的语义化简写。
func RequireAdmin() gin.HandlerFunc { return RequireRole("admin") }

// ---- API Key 认证 ----

// APIKeyResolver 由上层注入的解析函数：把明文 key 解析成身份。
// 返回 ok=false 表示密钥无效、被禁用或已过期。
// 抽象成函数类型，便于单元测试时替换为 mock。
type APIKeyResolver func(ctx context.Context, plain string) (id uint, userID uint, role string, ok bool)

// APIKey 校验 X-API-Key 请求头。
// 通过后写入 api_key_id，并沿用该 Key 归属用户的 user_id，
// 使机器调用与人工调用共享同一套 RBAC 判定。
func APIKey(resolve APIKeyResolver) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw := strings.TrimSpace(c.GetHeader("X-API-Key"))
		if raw == "" {
			// 兼容 Authorization: ApiKey <token> 写法
			if h := c.GetHeader("Authorization"); strings.HasPrefix(strings.ToLower(h), "apikey ") {
				raw = strings.TrimSpace(h[7:])
			}
		}
		if raw == "" {
			abort(c, apperr.Unauthorized("缺少 X-API-Key"))
			return
		}

		id, userID, role, ok := resolve(c.Request.Context(), raw)
		if !ok {
			abort(c, apperr.Unauthorized("API Key 无效或已禁用"))
			return
		}

		c.Set(string(KeyAPIKeyID), id)
		c.Set(string(KeyUserID), userID)
		c.Set(string(KeyRole), role)
		c.Next()
	}
}

// abort 统一的中断 + 响应写入。
func abort(c *gin.Context, err error) {
	c.Abort()
	response.Fail(c, err)
}

// ---- 限流 ----

// RateLimiter 基于令牌桶的进程内限流（按客户端 IP）。
//
// MVP 用内存桶即可；多实例部署时应换成基于 Redis 的分布式限流，
// 否则每个实例各算各的，实际放行量会随实例数放大。
func RateLimiter(r rate.Limit, burst int) gin.HandlerFunc {
	// 单独加锁保护 map，避免并发读写导致 panic。
	buckets := newBucketMap(r, burst)
	return func(c *gin.Context) {
		if !buckets.allow(c.ClientIP()) {
			abort(c, apperr.New(apperr.CodeRateLimited, "请求过于频繁，请稍后重试"))
			return
		}
		c.Next()
	}
}
