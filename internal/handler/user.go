package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"go-backend/internal/apperr"
	"go-backend/internal/middleware"
	_ "go-backend/internal/model"
	"go-backend/internal/model/dto"
	"go-backend/internal/response"
	"go-backend/internal/service"
)

// UserHandler 用户管理路由（需管理员权限）。
type UserHandler struct {
	users *service.UserService
}

// NewUserHandler 构造用户处理器。
func NewUserHandler(users *service.UserService) *UserHandler {
	return &UserHandler{users: users}
}

func (h *UserHandler) List(c *gin.Context) {
	var q dto.ListUserRequest
	if err := c.ShouldBindQuery(&q); err != nil {
		response.Fail(c, apperr.BadRequest(bindErrorMessage(err)))
		return
	}

	list, total, err := h.users.ListUser(c.Request.Context(), q)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Page(c, list, total, q.Page, q.PageSize)
}

func (h *UserHandler) Create(c *gin.Context) {
	var req dto.CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, apperr.BadRequest(bindErrorMessage(err)))
		return
	}

	u, err := h.users.CreateUser(c.Request.Context(), &req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, u)
}

func (h *UserHandler) Update(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	var req dto.UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, apperr.BadRequest(bindErrorMessage(err)))
		return
	}

	u, err := h.users.UpdateUser(c.Request.Context(), id, &req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, u)
}

func (h *UserHandler) Delete(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	operator := middleware.GetUserID(c)
	if err := h.users.DeleteUser(c.Request.Context(), id, operator); err != nil {
		response.Fail(c, err)
		return
	}
	response.OKWithMessage(c, "删除成功", nil)
}

func (h *UserHandler) Get(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	u, err := h.users.GetByID(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, u)
}

// ---- API Key ----

// APIKeyHandler API Key 管理路由。
type APIKeyHandler struct {
	keys *service.APIKeyService
}

// NewAPIKeyHandler 构造 API Key 处理器。
func NewAPIKeyHandler(keys *service.APIKeyService) *APIKeyHandler {
	return &APIKeyHandler{keys: keys}
}

func (h *APIKeyHandler) List(c *gin.Context) {
	var q dto.ListAPIKeyRequest
	if err := c.ShouldBindQuery(&q); err != nil {
		response.Fail(c, apperr.BadRequest(bindErrorMessage(err)))
		return
	}

	uid := middleware.GetUserID(c)
	list, total, err := h.keys.List(c.Request.Context(), uid, q)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Page(c, list, total, q.Page, q.PageSize)
}

func (h *APIKeyHandler) Create(c *gin.Context) {
	var req dto.CreateAPIKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, apperr.BadRequest(bindErrorMessage(err)))
		return
	}

	uid := middleware.GetUserID(c)
	res, err := h.keys.Create(c.Request.Context(), uid, &req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, res)
}

// operatorFrom 上下文中的身份构造归属校验用的 Operator。
func operatorFrom(c *gin.Context) *service.Operator {
	return &service.Operator{
		ID:   middleware.GetUserID(c),
		Role: middleware.GetRole(c),
	}
}

func (h *APIKeyHandler) Update(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	var body struct {
		Enabled *bool `json:"enabled" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, apperr.BadRequest("缺少 enabled 字段"))
		return
	}

	if err := h.keys.SetEnabled(c.Request.Context(), id, *body.Enabled, operatorFrom(c)); err != nil {
		response.Fail(c, err)
		return
	}
	response.OKWithMessage(c, "状态已更新", nil)
}

func (h *APIKeyHandler) Delete(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	if err := h.keys.Delete(c.Request.Context(), id, operatorFrom(c)); err != nil {
		response.Fail(c, err)
		return
	}
	response.OKWithMessage(c, "删除成功", nil)
}

// ---- 公共辅助 ----

// parseID 解析路径参数中的整型 ID，失败时已写入响应并返回 ok=false。
func parseID(c *gin.Context) (uint, bool) {
	v := c.Param("id")
	n, err := strconv.ParseUint(v, 10, 32)
	if err != nil || n == 0 {
		response.Fail(c, apperr.BadRequest("ID 必须是正整数"))
		return 0, false
	}
	return uint(n), true
}
