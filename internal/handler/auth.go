// Package handler HTTP 处理层（Controller）。
//
// 每个 handler 只做四件事：
//  1. 绑定并校验请求参数（binding tag 自动完成）
//  2. 调用 service 完成业务
//  3. 用 response 包封装统一格式
//
// 不在这一层写业务规则，也不直接操作数据库。
//
// API 文档不在这里声明：spec 由 docs 包以 Go struct 手写（OpenAPI 3.1），
// 路由变更时同步更新 docs/paths.go。
package handler

import (
	"errors"

	"github.com/gin-gonic/gin"

	"go-backend/internal/apperr"
	"go-backend/internal/middleware"
	_ "go-backend/internal/model"
	"go-backend/internal/model/dto"
	"go-backend/internal/response"
	"go-backend/internal/service"
)

// AuthHandler 认证相关路由。
type AuthHandler struct {
	users *service.UserService
}

// NewAuthHandler 构造认证处理器。
func NewAuthHandler(users *service.UserService) *AuthHandler {
	return &AuthHandler{users: users}
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req dto.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, apperr.BadRequest(bindErrorMessage(err)))
		return
	}

	res, err := h.users.Login(c.Request.Context(), &req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, res)
}

func (h *AuthHandler) Refresh(c *gin.Context) {
	var req dto.RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, apperr.BadRequest(bindErrorMessage(err)))
		return
	}

	pair, err := h.users.Refresh(c.Request.Context(), req.RefreshToken)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, pair)
}

func (h *AuthHandler) Me(c *gin.Context) {
	// 简化实现：从令牌里拿到的 ID 反查用户。
	// 后续接入用户仓储后可替换为直接读取。
	uid := middleware.GetUserID(c)
	if uid == 0 {
		response.Fail(c, apperr.Unauthorized("未登录"))
		return
	}
	user, err := h.users.GetByID(c.Request.Context(), uid)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, user)
}

func (h *AuthHandler) ChangePassword(c *gin.Context) {
	var req dto.ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, apperr.BadRequest(bindErrorMessage(err)))
		return
	}

	uid := middleware.GetUserID(c)
	if err := h.users.ChangePassword(c.Request.Context(), uid, &req); err != nil {
		response.Fail(c, err)
		return
	}
	response.OKWithMessage(c, "密码修改成功，请重新登录", nil)
}

// bindErrorMessage 把 binding 错误翻译成中文，
// 让前端/调用方直接拿到可读的原因而不是英文字段名。
func bindErrorMessage(err error) string {
	if err == nil {
		return "参数错误"
	}
	var ve interface{ Field() string }
	if errors.As(err, &ve) && ve.Field() != "" {
		return "参数 " + ve.Field() + " 校验失败"
	}
	return "请求参数格式错误"
}
