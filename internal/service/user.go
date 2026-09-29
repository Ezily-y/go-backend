// Package service 业务逻辑层。
//
// 位置：Handler 只做参数绑定与响应封装，Repository 只做数据读写，
// 其余所有业务规则（登录校验、角色变更约束、密钥生命周期等）都集中在这里。
package service

import (
	"context"
	"errors"

	"go-backend/internal/apperr"
	"go-backend/internal/auth"
	"go-backend/internal/model"
	"go-backend/internal/model/dto"
	"go-backend/internal/repository"
)

// UserService 用户与认证相关业务。
type UserService struct {
	users repository.UserRepository
	jwt   *auth.Service
}

// NewUserService 组装用户服务。
func NewUserService(users repository.UserRepository, jwt *auth.Service) *UserService {
	return &UserService{users: users, jwt: jwt}
}

// Login 校验用户名密码并签发令牌对。
func (s *UserService) Login(ctx context.Context, req *dto.LoginRequest) (*dto.LoginResponse, error) {
	u, err := s.users.GetByUsername(ctx, req.Username)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			// 用户不存在与密码错误返回同一提示，避免账号枚举。
			return nil, apperr.New(apperr.CodeBadCredentials, "用户名或密码错误")
		}
		return nil, err
	}
	if !auth.VerifyPassword(u.PasswordHash, req.Password) {
		return nil, apperr.New(apperr.CodeBadCredentials, "用户名或密码错误")
	}
	if u.Status != 1 {
		return nil, apperr.New(apperr.CodeForbidden, "账号已被禁用，请联系管理员")
	}

	tokens, err := s.jwt.Issue(u)
	if err != nil {
		return nil, err
	}
	return &dto.LoginResponse{
		TokenPair: pairOf(tokens),
		User:      u,
	}, nil
}

// Refresh 用刷新令牌换取新的令牌对。
// 这里会回查用户，确保已被禁用的账号无法继续续期。
func (s *UserService) Refresh(ctx context.Context, refreshToken string) (*dto.TokenPair, error) {
	claims, err := s.jwt.ParseWithExpectation(refreshToken, auth.TokenRefresh)
	if err != nil {
		return nil, apperr.Unauthorized("刷新令牌无效或已过期")
	}

	u, err := s.users.GetByID(ctx, claims.UserID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, apperr.Unauthorized("用户不存在或已被删除")
		}
		return nil, err
	}
	if u.Status != 1 {
		return nil, apperr.Forbidden("账号已被禁用")
	}

	tokens, err := s.jwt.Issue(u)
	if err != nil {
		return nil, err
	}
	p := pairOf(tokens)
	return &p, nil
}

// ChangePassword 修改自身密码。修改成功后应让客户端主动退出重新登录。
func (s *UserService) ChangePassword(ctx context.Context, userID uint, req *dto.ChangePasswordRequest) error {
	u, err := s.users.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return apperr.NotFound("用户不存在")
		}
		return err
	}
	if !auth.VerifyPassword(u.PasswordHash, req.OldPassword) {
		return apperr.New(apperr.CodeBadCredentials, "原密码错误")
	}
	if req.OldPassword == req.NewPassword {
		return apperr.BadRequest("新密码不能与原密码相同")
	}

	hash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		return apperr.NewWrap(apperr.CodeEncryptError, "密码处理失败", err)
	}
	return s.users.UpdatePassword(ctx, userID, hash)
}

// CreateUser 创建用户（管理员操作）。
func (s *UserService) CreateUser(ctx context.Context, req *dto.CreateUserRequest) (*model.User, error) {
	// 先做一次存在性检查，给出比「数据库唯一索引冲突」更友好的提示。
	if _, err := s.users.GetByUsername(ctx, req.Username); err == nil {
		return nil, apperr.New(apperr.CodeUserExists, "用户名已存在")
	} else if !errors.Is(err, repository.ErrNotFound) {
		return nil, err
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		return nil, apperr.NewWrap(apperr.CodeEncryptError, "密码处理失败", err)
	}

	u := &model.User{
		Username:     req.Username,
		PasswordHash: hash,
		Nickname:     req.Nickname,
		Role:         req.Role,
		Email:        req.Email,
		Status:       1,
	}
	if err := s.users.Create(ctx, u); err != nil {
		return nil, err
	}
	return u, nil
}

// UpdateUser 更新用户基础信息。
func (s *UserService) UpdateUser(ctx context.Context, id uint, req *dto.UpdateUserRequest) (*model.User, error) {
	u, err := s.users.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, apperr.NotFound("用户不存在")
		}
		return nil, err
	}

	// 指针字段表示「是否传入」，nil 即不修改。
	if req.Nickname != nil {
		u.Nickname = *req.Nickname
	}
	if req.Email != nil {
		u.Email = *req.Email
	}
	if req.Role != nil {
		u.Role = *req.Role
	}
	if req.Status != nil {
		u.Status = *req.Status
	}

	if err := s.users.Update(ctx, u); err != nil {
		return nil, err
	}
	return u, nil
}

// DeleteUser 删除用户，禁止删除自己（否则会把自己锁在门外）。
func (s *UserService) DeleteUser(ctx context.Context, targetID, operatorID uint) error {
	if targetID == operatorID {
		return apperr.BadRequest("不能删除当前登录的账号")
	}
	if err := s.users.Delete(ctx, targetID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return apperr.NotFound("用户不存在")
		}
		return err
	}
	return nil
}

// ListUser 分页查询用户。
func (s *UserService) ListUser(ctx context.Context, q dto.ListUserRequest) ([]*model.User, int64, error) {
	return s.users.List(ctx, q)
}

// GetByID 按 ID 查询单个用户。
func (s *UserService) GetByID(ctx context.Context, id uint) (*model.User, error) {
	u, err := s.users.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, apperr.NotFound("用户不存在")
		}
		return nil, err
	}
	return u, nil
}

// pairOf 把令牌结构转换成对外的 TokenPair。
func pairOf(t *auth.TokenStrings) dto.TokenPair {
	return dto.TokenPair{
		AccessToken:  t.AccessToken,
		RefreshToken: t.RefreshToken,
		ExpiresIn:    int64(t.AccessTTL.Seconds()),
		TokenType:    "Bearer",
	}
}
