package service

import (
	"context"
	"errors"
	"time"

	"go-backend/internal/core/apperr"
	"go-backend/internal/auth"
	"go-backend/internal/model"
	"go-backend/internal/model/dto"
	"go-backend/internal/repository"
)

// APIKeyService API Key 的签发与校验业务。
type APIKeyService struct {
	repo repository.APIKeyRepository
}

// NewAPIKeyService 组装 API Key 服务。
func NewAPIKeyService(repo repository.APIKeyRepository) *APIKeyService {
	return &APIKeyService{repo: repo}
}

// Create 签发新密钥。
// 明文只在本次返回值中出现一次，库里只保留前缀与哈希。
func (s *APIKeyService) Create(ctx context.Context, userID uint, req *dto.CreateAPIKeyRequest) (*dto.CreateAPIKeyResponse, error) {
	plain := auth.GenerateAPIKey()

	var expiredAt *time.Time
	if req.Days > 0 {
		t := nowUTC().Add(time.Duration(req.Days) * 24 * time.Hour)
		expiredAt = &t
	}

	k := &model.APIKey{
		Name:      req.Name,
		KeyPrefix: auth.KeyPrefix(plain),
		KeyHash:   auth.HashAPIKey(plain),
		UserID:    userID,
		Role:      req.Role,
		Enabled:   true,
		ExpiredAt: expiredAt,
	}
	if err := s.repo.Create(ctx, k); err != nil {
		return nil, err
	}

	resp := &dto.CreateAPIKeyResponse{
		ID:        k.ID,
		Name:      k.Name,
		Key:       plain,
		KeyPrefix: k.KeyPrefix,
	}
	if expiredAt != nil {
		resp.ExpiredAt = expiredAt.Format(time.RFC3339)
	}
	return resp, nil
}

// List 返回当前用户的密钥列表（不含明文）。
func (s *APIKeyService) List(ctx context.Context, userID uint, q dto.ListAPIKeyRequest) ([]*model.APIKey, int64, error) {
	return s.repo.ListByUser(ctx, userID, q)
}

// SetEnabled 启用/禁用指定密钥。operatorRole 为 nil 表示不做权限判断。
func (s *APIKeyService) SetEnabled(ctx context.Context, keyID uint, enabled bool, operator *Operator) error {
	if _, err := s.authorize(ctx, keyID, operator); err != nil {
		return err
	}
	if err := s.repo.SetEnabled(ctx, keyID, enabled); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return apperr.NotFound("API Key 不存在")
		}
		return err
	}
	return nil
}

// Delete 删除指定密钥。
func (s *APIKeyService) Delete(ctx context.Context, keyID uint, operator *Operator) error {
	if _, err := s.authorize(ctx, keyID, operator); err != nil {
		return err
	}
	if err := s.repo.Delete(ctx, keyID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return apperr.NotFound("API Key 不存在")
		}
		return err
	}
	return nil
}

// Get 按 ID 获取密钥（不含明文），并附带归属校验。
func (s *APIKeyService) Get(ctx context.Context, keyID uint, operator *Operator) (*model.APIKey, error) {
	return s.authorize(ctx, keyID, operator)
}

// Operator 描述发起操作的主体，用于归属校验。
type Operator struct {
	ID   uint   // 用户 ID
	Role string // 角色，admin 可跨用户操作
}

// authorize 取出密钥并校验当前操作者是否有权访问它。
// 非 admin 只能操作自己的密钥；越权时返回 404 而非 403，
// 避免向探测者泄露「该 ID 确实存在」这一事实。
func (s *APIKeyService) authorize(ctx context.Context, keyID uint, op *Operator) (*model.APIKey, error) {
	k, err := s.repo.GetByID(ctx, keyID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, apperr.NotFound("API Key 不存在")
		}
		return nil, err
	}
	if op == nil {
		return k, nil
	}
	if op.Role == string(model.RoleAdmin) || k.UserID == op.ID {
		return k, nil
	}
	return nil, apperr.NotFound("API Key 不存在")
}

// Resolve 供鉴权中间件调用：把明文 key 解析成身份信息。
// 返回 ok=false 表示无效；同时负责校验启用状态与有效期。
func (s *APIKeyService) Resolve(ctx context.Context, plain string) (id, userID uint, role string, ok bool) {
	// 先取出明文前缀，用索引快速定位候选记录。
	prefix := auth.KeyPrefix(plain)
	if prefix == "" {
		return 0, 0, "", false
	}

	k, err := s.repo.GetByPrefix(ctx, prefix)
	if err != nil {
		return 0, 0, "", false
	}
	// 定位到候选后，必须做完整哈希比对，防止仅凭前缀伪造。
	if !auth.VerifyAPIKey(k.KeyHash, plain) {
		return 0, 0, "", false
	}
	if !k.Enabled || k.IsExpired(nowUTC()) {
		return 0, 0, "", false
	}

	// 记录最后使用时间；失败不影响鉴权结果。
	_ = s.repo.TouchLastUsed(ctx, k.ID)

	return k.ID, k.UserID, string(k.Role), true
}

// nowUTC 返回当前 UTC 时间，统一时间基准，避免本机时区与数据库时区混用。
var nowUTC = func() time.Time { return time.Now().UTC() }
