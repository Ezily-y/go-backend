package repository

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"go-backend/internal/apperr"
	"go-backend/internal/database"
	"go-backend/internal/model"
	"go-backend/internal/model/dto"
)

// apiKeyRepo GORM 版 API Key 仓储。
type apiKeyRepo struct{ db *gorm.DB }

// NewAPIKeyRepo 创建 API Key 仓储。
func NewAPIKeyRepo(db *gorm.DB) APIKeyRepository {
	if db == nil {
		db = database.Get()
	}
	return &apiKeyRepo{db: db}
}

// Create 写入密钥记录。
func (r *apiKeyRepo) Create(ctx context.Context, k *model.APIKey) error {
	if err := r.db.WithContext(ctx).Create(k).Error; err != nil {
		return apperr.DB("创建 API Key 失败", err)
	}
	return nil
}

// GetByID 按主键查询。
func (r *apiKeyRepo) GetByID(ctx context.Context, id uint) (*model.APIKey, error) {
	var k model.APIKey
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&k).Error
	return wrapNotFound(&k, err, "API Key 不存在")
}

// GetByPrefix 按前缀查询。
// 前缀有索引，命中后再由调用方做完整哈希比对，
// 这样一次查询即可完成「定位 + 验证」。
func (r *apiKeyRepo) GetByPrefix(ctx context.Context, prefix string) (*model.APIKey, error) {
	var k model.APIKey
	err := r.db.WithContext(ctx).Where("key_prefix = ?", prefix).First(&k).Error
	return wrapNotFound(&k, err, "API Key 不存在")
}

// ListByUser 分页查询某用户下的密钥。
func (r *apiKeyRepo) ListByUser(ctx context.Context, userID uint, q dto.ListAPIKeyRequest) ([]*model.APIKey, int64, error) {
	q.Normalize()

	tx := r.db.WithContext(ctx).Model(&model.APIKey{}).Where("user_id = ?", userID)
	if kw := q.Keyword; kw != "" {
		tx = tx.Where("name LIKE ? OR key_prefix LIKE ?", "%"+kw+"%", "%"+kw+"%")
	}

	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, apperr.DB("统计 API Key 失败", err)
	}

	var list []*model.APIKey
	err := tx.Order("id DESC").
		Offset(q.Offset()).
		Limit(q.PageSize).
		Find(&list).Error
	if err != nil {
		return nil, 0, apperr.DB("查询 API Key 列表失败", err)
	}
	return list, total, nil
}

// SetEnabled 启用/禁用密钥。
func (r *apiKeyRepo) SetEnabled(ctx context.Context, id uint, enabled bool) error {
	res := r.db.WithContext(ctx).Model(&model.APIKey{}).
		Where("id = ?", id).
		Update("enabled", enabled)
	if res.Error != nil {
		return apperr.DB("更新 API Key 状态失败", res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// TouchLastUsed 记录最后使用时间。
// 单独拆成一条 UPDATE 而不是复用主流程，是为了让鉴权路径的读写解耦；
// 失败只降级为普通错误，不影响鉴权结果。
func (r *apiKeyRepo) TouchLastUsed(ctx context.Context, id uint) error {
	now := time.Now().UTC()
	err := r.db.WithContext(ctx).Model(&model.APIKey{}).
		Where("id = ?", id).
		Update("last_used", now).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return apperr.DB("更新 API Key 使用时间失败", err)
	}
	return nil
}

// Delete 删除密钥。
func (r *apiKeyRepo) Delete(ctx context.Context, id uint) error {
	res := r.db.WithContext(ctx).Where("id = ?", id).Delete(&model.APIKey{})
	if res.Error != nil {
		return apperr.DB("删除 API Key 失败", res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// ---- 调用日志 ----

// apiLogRepo GORM 版调用日志仓储。
type apiLogRepo struct{ db *gorm.DB }

// NewAPILogRepo 创建调用日志仓储。
func NewAPILogRepo(db *gorm.DB) APILogRepository {
	if db == nil {
		db = database.Get()
	}
	return &apiLogRepo{db: db}
}

// Create 写入一条调用日志。
// 监控日志属于尽力而为：失败时只返回错误，不阻断主业务流程。
func (r *apiLogRepo) Create(ctx context.Context, l *model.APILog) error {
	if err := r.db.WithContext(ctx).Create(l).Error; err != nil {
		return apperr.DB("写入调用日志失败", err)
	}
	return nil
}

// ListRecent 返回最近若干条调用记录，按时间倒序。
func (r *apiLogRepo) ListRecent(ctx context.Context, limit int) ([]*model.APILog, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}
	var list []*model.APILog
	err := r.db.WithContext(ctx).
		Order("id DESC").
		Limit(limit).
		Find(&list).Error
	if err != nil {
		return nil, apperr.DB("查询调用日志失败", err)
	}
	return list, nil
}

// CountByProvider 统计某个上游被调用的总次数。
func (r *apiLogRepo) CountByProvider(ctx context.Context, provider string) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).Model(&model.APILog{}).
		Where("provider = ?", provider).
		Count(&n).Error
	if err != nil {
		return 0, apperr.DB("统计调用次数失败", err)
	}
	return n, nil
}
