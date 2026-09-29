// Package repository 数据访问层。
//
// 职责：只做数据读写与查询拼装，不包含业务规则判断。
// 每个仓储先定义 interface（便于 service 层 mock 测试），再给出 GORM 实现。
package repository

import (
	"context"

	"go-backend/internal/model"
	"go-backend/internal/model/dto"
)

// ---- 用户 ----

// UserRepository 用户仓储接口。
type UserRepository interface {
	Create(ctx context.Context, u *model.User) error
	GetByID(ctx context.Context, id uint) (*model.User, error)
	GetByUsername(ctx context.Context, username string) (*model.User, error)
	Update(ctx context.Context, u *model.User) error
	Delete(ctx context.Context, id uint) error
	List(ctx context.Context, q dto.ListUserRequest) ([]*model.User, int64, error)
	Count(ctx context.Context) (int64, error)
	// UpdatePassword 单独更新密码哈希，与 Update 的字段集分离，避免互相覆盖。
	UpdatePassword(ctx context.Context, id uint, hash string) error
}

// ---- API Key ----

// APIKeyRepository API Key 仓储接口。
type APIKeyRepository interface {
	Create(ctx context.Context, k *model.APIKey) error
	GetByID(ctx context.Context, id uint) (*model.APIKey, error)
	GetByPrefix(ctx context.Context, prefix string) (*model.APIKey, error)
	ListByUser(ctx context.Context, userID uint, q dto.ListAPIKeyRequest) ([]*model.APIKey, int64, error)
	SetEnabled(ctx context.Context, id uint, enabled bool) error
	TouchLastUsed(ctx context.Context, id uint) error
	Delete(ctx context.Context, id uint) error
}

// ---- 第三方调用日志 ----

// APILogRepository 第三方 API 调用日志仓储接口。
type APILogRepository interface {
	Create(ctx context.Context, l *model.APILog) error
	ListRecent(ctx context.Context, limit int) ([]*model.APILog, error)
	CountByProvider(ctx context.Context, provider string) (int64, error)
}
