// Package dto 定义 HTTP 请求/响应的数据传输对象。
// 与持久化实体（model）分离，避免数据库字段变更直接影响 API 契约。
package dto

import (
	"go-backend/internal/model"
)

// ---- 认证 ----

// LoginRequest 登录请求。
type LoginRequest struct {
	Username string `json:"username" binding:"required,min=3,max=64" example:"admin"`
	Password string `json:"password" binding:"required,min=6,max=128" example:"admin123456"`
}

// TokenPair 令牌对：访问令牌 + 刷新令牌。
type TokenPair struct {
	AccessToken  string `json:"access_token" example:"eyJhbGciOi..."`
	RefreshToken string `json:"refresh_token" example:"eyJhbGciOi..."`
	ExpiresIn    int64  `json:"expires_in" example:"7200"` // 访问令牌有效期（秒）
	TokenType    string `json:"token_type" example:"Bearer"`
}

// LoginResponse 登录成功响应。
type LoginResponse struct {
	TokenPair
	User *model.User `json:"user"`
}

// RefreshRequest 刷新令牌请求。
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required" example:"eyJhbGciOi..."`
}

// ChangePasswordRequest 修改密码请求。
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required,min=6,max=128" example:"admin123456"`
	NewPassword string `json:"new_password" binding:"required,min=6,max=128" example:"newpass123456"`
}

// ---- 用户 ----

// CreateUserRequest 创建用户请求（管理员用）。
type CreateUserRequest struct {
	Username string     `json:"username" binding:"required,min=3,max=64" example:"alice"`
	Password string     `json:"password" binding:"required,min=6,max=128" example:"pass123456"`
	Nickname string     `json:"nickname" binding:"max=64" example:"爱丽丝"`
	Role     model.Role `json:"role" binding:"required,oneof=admin editor viewer" example:"viewer"`
	Email    string     `json:"email" binding:"omitempty,email" example:"alice@example.com"`
}

// UpdateUserRequest 更新用户请求；字段为空则不修改。
type UpdateUserRequest struct {
	Nickname *string     `json:"nickname" binding:"omitempty,max=64" example:"新昵称"`
	Role     *model.Role `json:"role" binding:"omitempty,oneof=admin editor viewer" example:"editor"`
	Email    *string     `json:"email" binding:"omitempty,email" example:"alice@example.com"`
	Status   *int8       `json:"status" binding:"omitempty,oneof=0 1" example:"1"`
}

// ---- API Key ----

// CreateAPIKeyRequest 创建 API Key 请求。
type CreateAPIKeyRequest struct {
	Name string     `json:"name" binding:"required,min=2,max=64" example:"my-tool"`
	Role model.Role `json:"role" binding:"required,oneof=admin editor viewer" example:"viewer"`
	Days int        `json:"days" binding:"omitempty,min=1,max=3650" example:"90"` // 有效天数，0 表示永不过期
}

// CreateAPIKeyResponse 创建 API Key 响应。
// key 字段只在此处出现一次，之后无法再次获取明文。
type CreateAPIKeyResponse struct {
	ID        uint   `json:"id" example:"1"`
	Name      string `json:"name" example:"my-tool"`
	Key       string `json:"key" example:"gk_ab12cd34..."`
	KeyPrefix string `json:"key_prefix" example:"gk_ab12"`
	ExpiredAt string `json:"expired_at,omitempty" example:"2026-12-28T00:00:00Z"`
}

// ---- 通用分页 ----

// PageQuery 通用分页查询参数，所有列表接口复用。
type PageQuery struct {
	Page     int `form:"page" json:"page" binding:"omitempty,min=1" example:"1"`
	PageSize int `form:"page_size" json:"page_size" binding:"omitempty,min=1,max=100" example:"20"`
}

// Normalize 将分页参数归一化，避免传入 0 或超大值。
func (q *PageQuery) Normalize() {
	if q.Page <= 0 {
		q.Page = 1
	}
	if q.PageSize <= 0 {
		q.PageSize = 20
	}
	if q.PageSize > 100 {
		q.PageSize = 100
	}
}

// Offset 计算 SQL LIMIT 的偏移量。
func (q *PageQuery) Offset() int { return (q.Page - 1) * q.PageSize }

// ListUserRequest 用户列表查询。
type ListUserRequest struct {
	PageQuery
	Keyword string `form:"keyword" json:"keyword" binding:"max=64" example:"adm"` // 模糊匹配用户名/昵称
	Role    string `form:"role" json:"role" binding:"omitempty,oneof=admin editor viewer" example:"admin"`
}

// ListAPIKeyRequest API Key 列表查询。
type ListAPIKeyRequest struct {
	PageQuery
	Keyword string `form:"keyword" json:"keyword" binding:"max=64" example:"tool"`
}
