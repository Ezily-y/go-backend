package auth

import (
	"strings"
	"testing"
	"time"

	"go-backend/internal/model"
)

// testSecret 满足「长度 >= 32」的校验要求。
const testSecret = "test-secret-0123456789-0123456789-ab"

// newTestService 构造测试用认证服务。
func newTestService(t *testing.T) *Service {
	t.Helper()
	s, err := New(testSecret, time.Hour, 24*time.Hour)
	if err != nil {
		t.Fatalf("创建认证服务失败: %v", err)
	}
	return s
}

// TestNewRejectsShortSecret 确保过短的密钥在启动阶段就被拦下，
// 而不是带着弱密钥跑进生产。
func TestNewRejectsShortSecret(t *testing.T) {
	if _, err := New("short", time.Hour, time.Hour); err == nil {
		t.Error("短于 32 字符的密钥应当被拒绝")
	}
	if _, err := New(testSecret, 0, time.Hour); err == nil {
		t.Error("有效期为 0 应当被拒绝")
	}
}

// TestIssueAndParse 覆盖令牌签发 → 解析的完整往返。
func TestIssueAndParse(t *testing.T) {
	svc := newTestService(t)
	user := &model.User{ID: 7, Username: "alice", Role: model.RoleEditor}

	pair, err := svc.Issue(user)
	if err != nil {
		t.Fatalf("签发失败: %v", err)
	}
	if pair.AccessToken == "" || pair.RefreshToken == "" {
		t.Fatal("令牌不应为空")
	}
	if pair.AccessTTL != time.Hour {
		t.Errorf("AccessTTL = %v, 期望 1h", pair.AccessTTL)
	}
	// 两类令牌必须不同，否则可用访问令牌冒充刷新令牌
	if pair.AccessToken == pair.RefreshToken {
		t.Error("访问令牌与刷新令牌不应相同")
	}

	claims, err := svc.Parse(pair.AccessToken)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if claims.UserID != 7 || claims.Username != "alice" || claims.Role != model.RoleEditor {
		t.Errorf("声明内容不符: %+v", claims)
	}
	if claims.TokenType != TokenAccess {
		t.Errorf("typ = %q, 期望 %q", claims.TokenType, TokenAccess)
	}
}

// TestTokenTypeSeparation 确保访问令牌不能当刷新令牌用，反之亦然。
// 这是防止「拿短期令牌无限续期」攻击的关键约束。
func TestTokenTypeSeparation(t *testing.T) {
	svc := newTestService(t)
	pair, err := svc.Issue(&model.User{ID: 1, Username: "a"})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := svc.ParseWithExpectation(pair.AccessToken, TokenRefresh); err == nil {
		t.Error("访问令牌不应被接受为刷新令牌")
	}
	if _, err := svc.ParseWithExpectation(pair.RefreshToken, TokenAccess); err == nil {
		t.Error("刷新令牌不应被接受为访问令牌")
	}
}

// TestParseTamperedToken 确保篡改过的令牌被拒绝（签名校验有效）。
func TestParseTamperedToken(t *testing.T) {
	svc := newTestService(t)
	pair, _ := svc.Issue(&model.User{ID: 1, Username: "a"})

	// 篡改 payload 中的一段字符
	tampered := pair.AccessToken[:20] + "x" + pair.AccessToken[21:]
	if _, err := svc.Parse(tampered); err == nil {
		t.Error("被篡改的令牌应当校验失败")
	}
	if _, err := svc.Parse("not.a.token"); err == nil {
		t.Error("格式非法的令牌应当校验失败")
	}
	if _, err := svc.Parse(""); err == nil {
		t.Error("空令牌应当校验失败")
	}
}

// TestParseWithWrongSecret 用另一把密钥签的令牌必须被拒，
// 防止跨环境令牌被误接受。
func TestParseWithWrongSecret(t *testing.T) {
	other, err := New("another-secret-0123456789-0123456789-x", time.Hour, 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	foreign, err := other.Issue(&model.User{ID: 1, Username: "a"})
	if err != nil {
		t.Fatal(err)
	}

	svc := newTestService(t)
	if _, err := svc.Parse(foreign.AccessToken); err == nil {
		t.Error("其他密钥签发的令牌应当被拒绝")
	}
}

// TestExpiredToken 验证过期令牌无法使用。
// 有效期设为 2ms，等待其过期后解析应失败。
func TestExpiredToken(t *testing.T) {
	svc, err := New(testSecret, 2*time.Millisecond, 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	pair, err := svc.Issue(&model.User{ID: 1, Username: "a"})
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(20 * time.Millisecond)

	if _, err := svc.Parse(pair.AccessToken); err == nil {
		t.Error("已过期的令牌应当被拒绝")
	} else if !strings.Contains(err.Error(), "令牌校验失败") {
		t.Errorf("错误信息应为中文提示，实际: %v", err)
	}
}
