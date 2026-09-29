// Package auth 提供 JWT 令牌的签发、校验与刷新。
//
// 使用 HS256 对称签名，密钥来自配置（生产环境必须用环境变量 JWT_SECRET 覆盖）。
// 访问令牌与刷新令牌共用签名密钥，通过 Claims.TokenType 区分用途，
// 防止拿访问令牌去调用刷新接口。
package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"go-backend/internal/model"
)

// TokenType 令牌类型，写入 JWT 的 typ 声明。
type TokenType string

const (
	TokenAccess  TokenType = "access"  // 访问令牌，用于常规鉴权
	TokenRefresh TokenType = "refresh" // 刷新令牌，仅用于换取新的访问令牌
)

// Claims 在标准声明之上扩展业务字段。
// Role 一并写入令牌，鉴权时可直接使用，避免每个请求都查一次库。
type Claims struct {
	jwt.RegisteredClaims
	TokenType TokenType  `json:"typ"`   // access | refresh
	UserID    uint       `json:"uid"`   // 用户 ID
	Username  string     `json:"uname"` // 用户名，便于日志排查
	Role      model.Role `json:"role"`  // 角色
}

// TokenStrings 一对可直接下发给客户端的令牌。
type TokenStrings struct {
	AccessToken  string
	RefreshToken string
	AccessTTL    time.Duration // 访问令牌有效期，用于返回 expires_in
}

// Service 封装令牌签发与校验。
type Service struct {
	secret     []byte
	accessTTL  time.Duration
	refreshTTL time.Duration
	issuer     string
}

// New 创建认证服务。secret 过短会在配置校验阶段被拦截，这里再兜一道底。
func New(secret string, accessTTL, refreshTTL time.Duration) (*Service, error) {
	if len(secret) < 32 {
		return nil, errors.New("JWT 密钥长度必须 >= 32")
	}
	if accessTTL <= 0 || refreshTTL <= 0 {
		return nil, errors.New("令牌有效期必须为正数")
	}
	return &Service{
		secret:     []byte(secret),
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
		issuer:     "go-backend",
	}, nil
}

// Issue 签发一对令牌并返回字符串形式。
func (s *Service) Issue(u *model.User) (*TokenStrings, error) {
	now := time.Now()

	accessClaims := s.newClaims(u, TokenAccess, now.Add(s.accessTTL))
	refreshClaims := s.newClaims(u, TokenRefresh, now.Add(s.refreshTTL))

	access, err := s.sign(accessClaims)
	if err != nil {
		return nil, err
	}
	refresh, err := s.sign(refreshClaims)
	if err != nil {
		return nil, err
	}
	return &TokenStrings{
		AccessToken:  access,
		RefreshToken: refresh,
		AccessTTL:    s.accessTTL,
	}, nil
}

// newClaims 构造一组完整的业务声明。
func (s *Service) newClaims(u *model.User, typ TokenType, expires time.Time) *Claims {
	now := time.Now()
	return &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    s.issuer,
			Subject:   fmt.Sprintf("%d", u.ID),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now.Add(-time.Minute)), // 允许 1 分钟时钟漂移
			ExpiresAt: jwt.NewNumericDate(expires),
			ID:        model.NewID(8), // jti，便于将来做令牌吊销
		},
		TokenType: typ,
		UserID:    u.ID,
		Username:  u.Username,
		Role:      u.Role,
	}
}

// sign 用 HS256 签名令牌。
func (s *Service) sign(claims *Claims) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(s.secret)
	if err != nil {
		return "", fmt.Errorf("签发令牌失败: %w", err)
	}
	return signed, nil
}

// Parse 解析并校验访问令牌。
func (s *Service) Parse(token string) (*Claims, error) {
	return s.ParseWithExpectation(token, TokenAccess)
}

// ParseWithExpectation 解析令牌并校验类型，
// 避免用刷新令牌冒充访问令牌（或反之）。
func (s *Service) ParseWithExpectation(raw string, want TokenType) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(raw, claims, s.keyFunc, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	if err != nil {
		return nil, fmt.Errorf("令牌校验失败: %w", err)
	}
	if !token.Valid {
		return nil, errors.New("令牌无效")
	}
	if claims.TokenType != want {
		return nil, fmt.Errorf("令牌类型不匹配，期望 %s，实际 %s", want, claims.TokenType)
	}
	return claims, nil
}

// keyFunc 返回签名密钥，同时显式约束签名算法，
// 防御 alg=none / 算法混淆类攻击。
func (s *Service) keyFunc(t *jwt.Token) (interface{}, error) {
	if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
		return nil, fmt.Errorf("不支持的签名算法: %v", t.Header["alg"])
	}
	return s.secret, nil
}

// AccessTTL 返回访问令牌有效期。
func (s *Service) AccessTTL() time.Duration { return s.accessTTL }

// RefreshTTL 返回刷新令牌有效期。
func (s *Service) RefreshTTL() time.Duration { return s.refreshTTL }
