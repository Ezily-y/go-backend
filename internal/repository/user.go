package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"go-backend/internal/apperr"
	"go-backend/internal/database"
	"go-backend/internal/model"
	"go-backend/internal/model/dto"
)

// ErrNotFound 由仓储层统一返回的「记录不存在」错误。
// 上层用 errors.Is 判断，避免依赖 gorm 的具体错误。
var ErrNotFound = errors.New("record not found")

// ---- GORM 实现 ----

// userRepo GORM 版用户仓储。
type userRepo struct{ db *gorm.DB }

// NewUserRepo 创建用户仓储，db 为空时使用全局连接。
func NewUserRepo(db *gorm.DB) UserRepository {
	if db == nil {
		db = database.Get()
	}
	return &userRepo{db: db}
}

// Create 写入用户；用户名重复时返回业务错误而非裸驱动错误。
func (r *userRepo) Create(ctx context.Context, u *model.User) error {
	err := r.db.WithContext(ctx).Create(u).Error
	if err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return apperr.New(apperr.CodeUserExists, "用户名已存在")
		}
		return apperr.DB("创建用户失败", err)
	}
	return nil
}

// GetByID 按主键查询，不存在时返回 ErrNotFound。
func (r *userRepo) GetByID(ctx context.Context, id uint) (*model.User, error) {
	var u model.User
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&u).Error
	return wrapNotFound(&u, err, "用户不存在")
}

// GetByUsername 按用户名查询（登录与唯一性校验都会用到）。
func (r *userRepo) GetByUsername(ctx context.Context, username string) (*model.User, error) {
	var u model.User
	err := r.db.WithContext(ctx).Where("username = ?", username).First(&u).Error
	return wrapNotFound(&u, err, "用户不存在")
}

// Update 更新用户的可变字段（用户名与密码不在此处修改）。
//
// 两个要点：
//   - 用 Select 显式列出待更新列，让零值也能被写入
//     （Updates 传结构体时默认跳过零值，status=0 会因此更新失败）；
//   - 更新成功后重新读回整行。GORM 只负责把 updated_at 写进数据库，
//     不会同步回传入的结构体，直接返回 u 会让响应里的时间戳停留在旧值。
func (r *userRepo) Update(ctx context.Context, u *model.User) error {
	if u.ID == 0 {
		return apperr.BadRequest("缺少用户 ID")
	}

	err := r.db.WithContext(ctx).
		Model(&model.User{}).
		Where("id = ?", u.ID).
		Select("nickname", "role", "email", "status").
		Updates(u).Error
	if err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return apperr.New(apperr.CodeUserExists, "用户名已存在")
		}
		return apperr.DB("更新用户失败", err)
	}

	if err := r.db.WithContext(ctx).Where("id = ?", u.ID).First(u).Error; err != nil {
		return apperr.DB("回读用户失败", err)
	}
	return nil
}

// UpdatePassword 单独更新密码哈希（与 Update 的字段集不同，避免互相干扰）。
func (r *userRepo) UpdatePassword(ctx context.Context, id uint, hash string) error {
	err := r.db.WithContext(ctx).Model(&model.User{}).
		Where("id = ?", id).
		Update("password_hash", hash).Error
	if err != nil {
		return apperr.DB("更新密码失败", err)
	}
	return nil
}

// Delete 按主键删除。
func (r *userRepo) Delete(ctx context.Context, id uint) error {
	res := r.db.WithContext(ctx).Where("id = ?", id).Delete(&model.User{})
	if res.Error != nil {
		return apperr.DB("删除用户失败", res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// List 分页查询用户，支持关键字与角色过滤。
func (r *userRepo) List(ctx context.Context, q dto.ListUserRequest) ([]*model.User, int64, error) {
	q.Normalize()

	tx := r.db.WithContext(ctx).Model(&model.User{})
	if kw := q.Keyword; kw != "" {
		like := "%" + kw + "%"
		tx = tx.Where("username LIKE ? OR nickname LIKE ?", like, like)
	}
	if q.Role != "" {
		tx = tx.Where("role = ?", q.Role)
	}

	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, apperr.DB("统计用户失败", err)
	}

	var list []*model.User
	err := tx.Order("id DESC").
		Offset(q.Offset()).
		Limit(q.PageSize).
		Find(&list).Error
	if err != nil {
		return nil, 0, apperr.DB("查询用户列表失败", err)
	}
	return list, total, nil
}

// Count 统计用户总数，用于首次启动时的种子判断。
func (r *userRepo) Count(ctx context.Context) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).Model(&model.User{}).Count(&n).Error
	if err != nil {
		return 0, apperr.DB("统计用户失败", err)
	}
	return n, nil
}

// wrapNotFound 统一把 gorm.ErrRecordNotFound 转成业务错误。
// 返回值固定三个，便于写成 `return wrapNotFound(...)` 的单行形式。
func wrapNotFound[T any](out *T, err error, msg string) (*T, error) {
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		return nil, ErrNotFound
	case err != nil:
		return nil, apperr.DB(msg, err)
	default:
		return out, nil
	}
}
