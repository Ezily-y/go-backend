// Package router 负责注册所有 HTTP 路由与中间件。
//
// 中间件顺序（全局）：
//
//	RequestID → Recovery → Logger → CORS → Metrics
//
// 鉴权按路由组分别挂载：公开区 / JWT 区 / API Key 区，
// 避免对健康检查、指标抓取这类端点也做鉴权。
package router

import (
	"time"

	"github.com/gin-gonic/gin"

	"go-backend/docs"
	"go-backend/internal/core/apperr"
	"go-backend/internal/auth"
	"go-backend/internal/core/config"
	"go-backend/internal/handler"
	"go-backend/internal/core/logger"
	"go-backend/internal/core/metrics"
	"go-backend/internal/middleware"
	"go-backend/internal/core/response"
	"go-backend/internal/service"
)

// Handlers 汇总所有 handler 与中间件依赖，作为路由注册的输入。
// 用结构体传递而非一长串参数，便于后续增删。
type Handlers struct {
	Auth   *handler.AuthHandler
	User   *handler.UserHandler
	APIKey *handler.APIKeyHandler
	System *handler.SystemHandler
	JWT    *auth.Service
	KeySVC *service.APIKeyService
}

// New 构建并返回已注册全部路由的 Engine。
func New(cfg *config.Config, h *Handlers) *gin.Engine {
	// release 模式下屏蔽 gin 自带的调试输出，日志统一交给 zap。
	if cfg.App.Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	e := gin.New()

	// ---- 全局中间件（顺序敏感）----
	e.Use(
		middleware.RequestID(), // 最先注入链路 ID，后续中间件都要用
		middleware.Recover(),   // 紧随其后，兜住后续所有 panic
		middleware.Logger(),    // 依赖 RequestID，需在其后
		middleware.CORS(cfg.App.CORSOrigins),
		metricsMiddleware(), // 记录 Prometheus 指标
	)

	// ---- 公开路由（无需鉴权）----
	e.GET("/health", h.System.Health)
	e.GET("/ready", h.System.Ready)
	e.GET("/metrics", metrics.Handler())
	// 静态文件直出；目录不存在时 gin 会返回 404，不会 panic
	e.Static("/uploads", cfg.App.UploadDir)

	registerDocs(e, cfg)

	// ---- API 分组 ----
	v1 := e.Group("/api/v1")

	// 公开区：登录与令牌刷新。登录挂限流，防止撞库。
	//
	// 注意中间件必须写在 handler 之前：gin 按参数顺序串起整条 handler 链，
	// 把限流放在后面的话，请求会先被 handler 处理完，限流形同虚设
	// （实测表现为永远返回参数校验 400 而非 429）。
	authGroup := v1.Group("/auth")
	authGroup.POST("/login", middleware.RateLimiter(1, 5), h.Auth.Login)
	authGroup.POST("/refresh", middleware.RateLimiter(1, 10), h.Auth.Refresh)

	// JWT 区：登录后的常规接口
	protected := v1.Group("", middleware.JWT(h.JWT))
	protected.GET("/auth/me", h.Auth.Me)
	protected.PUT("/auth/password", h.Auth.ChangePassword)
	protected.GET("/system/info", h.System.Info)

	// API Key 管理：任何登录用户都能管理自己的密钥
	protected.GET("/apikeys", h.APIKey.List)
	protected.POST("/apikeys", h.APIKey.Create)
	protected.PATCH("/apikeys/:id", h.APIKey.Update)
	protected.DELETE("/apikeys/:id", h.APIKey.Delete)

	// 用户管理：仅 admin 可见
	admin := v1.Group("/users", middleware.JWT(h.JWT), middleware.RequireAdmin())
	admin.GET("", h.User.List)
	admin.POST("", h.User.Create)
	admin.GET("/:id", h.User.Get)
	admin.PUT("/:id", h.User.Update)
	admin.DELETE("/:id", h.User.Delete)

	// API Key 区：供外部工具调用，与 JWT 区完全隔离。
	tool := v1.Group("/tool", middleware.APIKey(h.KeySVC.Resolve))
	// 示例端点，验证 API Key 鉴权链路是否打通
	tool.GET("/ping", func(c *gin.Context) {
		response.OK(c, gin.H{"pong": true, "role": middleware.GetRole(c)})
	})

	// 404 / 405 统一成标准响应体
	e.NoRoute(func(c *gin.Context) {
		response.Fail(c, apperr.NotFound("接口不存在"))
	})
	e.NoMethod(func(c *gin.Context) {
		response.FailWithCode(c, 405000, "请求方法不允许")
	})

	return e
}

// metricsMiddleware 把每个请求记录到 Prometheus。
// path 标签使用路由模板（c.FullPath）而非真实路径，
// 否则每个不同 ID 都会生成一条时间序列，造成指标基数爆炸。
func metricsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		path := c.FullPath()
		if path == "" {
			path = "unmatched"
		}
		metrics.ObserveRequest(c.Request.Method, path, c.Writer.Status(), time.Since(start).Seconds())
	}
}

// registerDocs 挂载 API 文档：Scalar UI + OpenAPI 3.1 spec。
//
// 是否注册只看 app.docs_enabled，与 app.mode 解耦：
//   - mode=debug/test 且未显式关闭 → 开（本地开发即开即用）
//   - mode=release → 默认关，避免暴露接口结构；确需对外用 DOCS_ENABLED=true 打开
//
// spec 由 docs 包在运行时构造（手写 Go struct），无生成步骤、无外部工具依赖。
func registerDocs(e *gin.Engine, cfg *config.Config) {
	if !cfg.App.DocsEnabled {
		logger.Debugf("API 文档已关闭（app.docs_enabled=false），如需开启设置 DOCS_ENABLED=true")
		return
	}
	e.GET("/docs", gin.WrapF(docs.UIHandler()))
	e.GET("/openapi.json", gin.WrapF(docs.SpecHandler()))
	logger.Infof("API 文档已启用: /docs  (spec: /openapi.json)")
}
