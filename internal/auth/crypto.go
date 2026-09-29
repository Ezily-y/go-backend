// Package auth 同时提供密码哈希与 API Key 的生成/校验工具。
//
// 密码使用 bcrypt（自带随机盐，无需手工处理）；
// API Key 采用「明文只返回一次 + 库里存哈希」的策略，与密码一致。
package auth

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"go-backend/internal/model"
)

// bcryptCost 平衡安全与性能：10 约等于每次哈希 50-100ms。
// 调高会显著拖慢登录接口，调低则降低暴力破解成本。
const bcryptCost = 10

// HashPassword 生成密码哈希。
func HashPassword(plain string) (string, error) {
	if plain == "" {
		return "", errors.New("密码不能为空")
	}
	b, err := bcrypt.GenerateFromPassword([]byte(plain), bcryptCost)
	if err != nil {
		return "", errors.New("密码哈希失败")
	}
	return string(b), nil
}

// VerifyPassword 比对明文与哈希。
// bcrypt 自带常数时间比较，不会因为返回时间差异泄露信息。
func VerifyPassword(hash, plain string) bool {
	if hash == "" || plain == "" {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}

// keyPrefix 生成的 API Key 前缀，便于在日志中识别。
const keyPrefix = "gk_"

// GenerateAPIKey 生成新的 API Key 明文。
// 形如 gk_<32字节随机hex>，共 3+64 = 67 字符。
// 使用 model.NewID 是安全的：model 包不依赖 auth，不存在循环引用。
func GenerateAPIKey() string {
	return keyPrefix + model.NewID(32)
}

// HashAPIKey 计算 API Key 的存储哈希。
// 这里使用 SHA-256 而非 bcrypt，原因是：
//   - key 本身是 32 字节高熵随机值，不存在字典攻击面，无需慢哈希；
//   - 校验频率高（每个机器调用都要走一次），需要快速且可常数时间比较。
func HashAPIKey(key string) string {
	sum := sha256.Sum256([]byte(normalizeKey(key)))
	return hex.EncodeToString(sum[:])
}

// VerifyAPIKey 常数时间比对明文与存储哈希，避免时序侧信道。
func VerifyAPIKey(hash, key string) bool {
	if hash == "" || key == "" {
		return false
	}
	computed := HashAPIKey(key)
	return subtle.ConstantTimeCompare([]byte(hash), []byte(computed)) == 1
}

// KeyPrefix 返回明文 key 的可展示前缀（含下划线，共 8 字符），
// 用于列表页识别是哪把 key，而无需暴露完整明文。
func KeyPrefix(key string) string {
	k := normalizeKey(key)
	if len(k) <= 8 {
		return k
	}
	return k[:8]
}

// normalizeKey 归一化：去除首尾空白，兼容 HTTP Header 中可能出现的空格。
func normalizeKey(key string) string {
	return strings.TrimSpace(key)
}
