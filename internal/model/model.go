// Package model 定义持久化实体与请求/响应 DTO。
//
// 约定：
//   - 持久化实体带 `db` 与 `gorm` 标签；
//   - 对外传输的结构体带 `json` 标签，文档 schema 由 docs 包手工对应；
//   - 时间统一使用 UTC。
package model

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

// Role 角色标识。MVP 只内置三种，后续接入 Casbin 后可动态扩展。
type Role string

const (
	RoleAdmin  Role = "admin"  // 超级管理员：拥有全部权限
	RoleEditor Role = "editor" // 编辑者：可读写业务数据
	RoleViewer Role = "viewer" // 访客：只读
)

// User 用户表。
type User struct {
	ID           uint      `json:"id" gorm:"primaryKey;autoIncrement" example:"1"`
	Username     string    `json:"username" gorm:"size:64;uniqueIndex:uk_users_username;not null" example:"admin"`
	PasswordHash string    `json:"-" gorm:"size:128;not null"` // 不出现在任何响应中
	Nickname     string    `json:"nickname" gorm:"size:64" example:"管理员"`
	Role         Role      `json:"role" gorm:"size:16;index;not null;default:viewer" example:"admin"`
	Email        string    `json:"email" gorm:"size:128" example:"admin@example.com"`
	Status       int8      `json:"status" gorm:"not null;default:1" example:"1"` // 1=启用 0=禁用
	CreatedAt    time.Time `json:"created_at" gorm:"not null"`
	UpdatedAt    time.Time `json:"updated_at" gorm:"not null"`
}

// TableName 指定表名，避免 GORM 使用复数形式带来的歧义。
func (User) TableName() string { return "users" }

// APIKey 供外部工具调用时使用的密钥。
//
// Key 只在创建时明文返回一次，库里只存前缀 + 哈希，
// 与用户密码的存储策略保持一致。
type APIKey struct {
	ID        uint       `json:"id" gorm:"primaryKey;autoIncrement" example:"1"`
	Name      string     `json:"name" gorm:"size:64;not null" example:"my-tool"`             // 别名，便于识别用途
	KeyPrefix string     `json:"key_prefix" gorm:"size:16;index;not null" example:"gk_ab12"` // 明文前缀，用于检索与展示
	KeyHash   string     `json:"-" gorm:"size:128;not null"`                                 // 完整 key 的哈希
	UserID    uint       `json:"user_id" gorm:"index;not null" example:"1"`                  // 归属用户
	Role      Role       `json:"role" gorm:"size:16;not null;default:viewer" example:"viewer"`
	Enabled   bool       `json:"enabled" gorm:"not null;default:true" example:"true"`
	LastUsed  *time.Time `json:"last_used,omitempty" example:"2026-01-01T00:00:00Z"`
	CreatedAt time.Time  `json:"created_at" gorm:"not null"`
	ExpiredAt *time.Time `json:"expired_at,omitempty" example:"2027-01-01T00:00:00Z"` // nil 表示永不过期
}

// TableName 指定表名。
func (APIKey) TableName() string { return "api_keys" }

// IsExpired 判断密钥是否已过期；永不过期的密钥始终返回 false。
func (k *APIKey) IsExpired(now time.Time) bool {
	return k.ExpiredAt != nil && now.After(*k.ExpiredAt)
}

// APILog 第三方 API 调用日志，用于监控与排障。
type APILog struct {
	ID         uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	Provider   string    `json:"provider" gorm:"size:64;index;not null" example:"openweather"`
	Endpoint   string    `json:"endpoint" gorm:"size:255;not null" example:"/data/2.5/weather"`
	StatusCode int       `json:"status_code" gorm:"not null" example:"200"`
	LatencyMS  int64     `json:"latency_ms" gorm:"not null" example:"120"`
	Cached     bool      `json:"cached" gorm:"not null;default:false" example:"false"`
	ErrorMsg   string    `json:"error_msg" gorm:"size:512" example:""`
	CreatedAt  time.Time `json:"created_at" gorm:"index;not null"`
}

// TableName 指定表名。
func (APILog) TableName() string { return "api_logs" }

// NewID 生成一段随机十六进制字符串，用于 API Key 明文等场景。
func NewID(n int) string {
	if n <= 0 {
		n = 16
	}
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		// crypto/rand 失败属于系统级异常，直接 panic 比静默降级更安全
		// （降级到可预测的 ID 会让密钥变得可猜测）。
		panic("生成随机 ID 失败: " + err.Error())
	}
	return hex.EncodeToString(buf)
}
