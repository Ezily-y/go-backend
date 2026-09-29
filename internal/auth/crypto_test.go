package auth

import (
	"strings"
	"testing"
)

// TestHashAndVerifyPassword 覆盖密码哈希的正反两路。
func TestHashAndVerifyPassword(t *testing.T) {
	hash, err := HashPassword("secret123")
	if err != nil {
		t.Fatalf("哈希失败: %v", err)
	}
	if !VerifyPassword(hash, "secret123") {
		t.Error("正确密码应当校验通过")
	}
	if VerifyPassword(hash, "wrong-password") {
		t.Error("错误密码应当校验失败")
	}
	// 明文绝不能出现在哈希里
	if strings.Contains(hash, "secret123") {
		t.Error("哈希值中不应包含明文密码")
	}
}

// TestHashPasswordIsSalted 相同密码两次哈希应产生不同结果
// （bcrypt 自带随机盐），否则彩虹表可一次性反查全部相同密码。
func TestHashPasswordIsSalted(t *testing.T) {
	h1, err := HashPassword("same-password")
	if err != nil {
		t.Fatal(err)
	}
	h2, err := HashPassword("same-password")
	if err != nil {
		t.Fatal(err)
	}
	if h1 == h2 {
		t.Error("相同密码应产生不同的哈希值（缺少随机盐）")
	}
}

// TestHashPasswordEdge 空密码等非法输入应报错而非静默生成哈希。
func TestHashPasswordEdge(t *testing.T) {
	if _, err := HashPassword(""); err == nil {
		t.Error("空密码应当返回错误")
	}
	if VerifyPassword("", "") {
		t.Error("两个空串不应判定为匹配")
	}
	if VerifyPassword("", "any") || VerifyPassword("hash", "") {
		t.Error("任一侧为空都应判定为不匹配")
	}
}

// TestGenerateAPIKey 验证密钥格式与随机性。
func TestGenerateAPIKey(t *testing.T) {
	k1 := GenerateAPIKey()
	k2 := GenerateAPIKey()

	if !strings.HasPrefix(k1, "gk_") {
		t.Errorf("密钥应以 gk_ 开头，实际 %q", k1)
	}
	// "gk_" + 32 字节随机数的十六进制（64 字符）= 67 字符
	if want := 3 + 64; len(k1) != want {
		t.Errorf("密钥长度 = %d, 期望 %d", len(k1), want)
	}
	if k1 == k2 {
		t.Error("两次生成的密钥不应相同")
	}
}

// TestHashAndVerifyAPIKey 验证密钥哈希与常数时间比对。
func TestHashAndVerifyAPIKey(t *testing.T) {
	key := GenerateAPIKey()
	hash := HashAPIKey(key)

	if strings.Contains(hash, key) {
		t.Error("存储哈希中不应包含完整明文")
	}
	if !VerifyAPIKey(hash, key) {
		t.Error("正确密钥应当校验通过")
	}
	if VerifyAPIKey(hash, key+"x") {
		t.Error("被改动的密钥应当校验失败")
	}
	// 前缀相同、后缀不同的情况必须失败。
	// 末位必须与原 key 真正不同：原 key 末位若恰好是 '0'，
	// 直接拼 "0" 会让近似密钥等于原密钥，本断言会以约 1/16 的概率误报失败
	// （GenerateAPIKey 产生的是随机 hex，末位为 0 的概率正是 1/16）。
	lastByte := "0"
	if key[len(key)-1] == '0' {
		lastByte = "1"
	}
	near := key[:len(key)-1] + lastByte
	if VerifyAPIKey(hash, near) {
		t.Errorf("同前缀的近似密钥不应通过校验（near=%q）", near)
	}
}

// TestKeyPrefix 验证展示用前缀的截取规则。
func TestKeyPrefix(t *testing.T) {
	key := GenerateAPIKey() // gk_ + 64 字符
	prefix := KeyPrefix(key)

	if prefix != key[:8] {
		t.Errorf("前缀 = %q, 期望 %q", prefix, key[:8])
	}
	if strings.Contains(key, prefix) == false {
		t.Error("前缀必然是原文的子串")
	}
	// 短字符串应原样返回，不能越界 panic
	if got := KeyPrefix("gk_ab"); got != "gk_ab" {
		t.Errorf("短字符串前缀 = %q, 期望原样返回", got)
	}
	if got := KeyPrefix(""); got != "" {
		t.Errorf("空字符串前缀 = %q, 期望空", got)
	}
}

// TestNormalizeKey 验证前后空白被清理，避免 Header 换行导致校验失败。
func TestNormalizeKey(t *testing.T) {
	key := GenerateAPIKey()
	if HashAPIKey("  "+key+"\n") != HashAPIKey(key) {
		t.Error("首尾空白应被清理后再哈希")
	}
}
