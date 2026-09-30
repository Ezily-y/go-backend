package apperr

import (
	"errors"
	"net/http"
	"testing"
)

// TestHTTPFromCode 验证「错误码千位前缀 = HTTP 状态码」这条规则。
// 这是整个错误码体系的地基，一旦算错，前端会收到语义错误的状态码。
func TestHTTPFromCode(t *testing.T) {
	cases := []struct {
		code int
		want int
	}{
		{0, http.StatusOK},
		{400001, http.StatusBadRequest},
		{401001, http.StatusUnauthorized},
		{401002, http.StatusUnauthorized},
		{403001, http.StatusForbidden},
		{404000, http.StatusNotFound},
		{405000, http.StatusMethodNotAllowed},
		{409001, http.StatusConflict},
		{429000, http.StatusTooManyRequests},
		{500001, http.StatusInternalServerError},
		{503000, http.StatusServiceUnavailable},
		{99, http.StatusInternalServerError}, // 非法码兜底为 500
	}
	for _, c := range cases {
		if got := httpFromCode(c.code); got != c.want {
			t.Errorf("httpFromCode(%d) = %d, 期望 %d", c.code, got, c.want)
		}
	}
}

// TestNewAndHTTPStatus 验证构造器能带上正确的 HTTP 状态码。
func TestNewAndHTTPStatus(t *testing.T) {
	if e := New(CodeInvalidParams, "参数错误"); e.HTTPStatus() != 400 {
		t.Errorf("参数错误应为 400，实际 %d", e.HTTPStatus())
	}
	if e := New(CodeUserExists, "用户名已存在"); e.HTTPStatus() != 409 {
		t.Errorf("用户名已存在应为 409，实际 %d", e.HTTPStatus())
	}

	// 未显式指定 HTTP 时应兜底为 500，而不是 0（0 会让 gin 崩溃）。
	if (&Error{Code: 123}).HTTPStatus() != http.StatusInternalServerError {
		t.Error("HTTP 为空时应兜底返回 500")
	}
}

// TestWrapPreservesOriginal 验证 Wrap 保存错误链且不修改原值。
// 不修改原值很关键：预定义错误常量若被就地污染，会影响后续所有调用。
func TestWrapPreservesOriginal(t *testing.T) {
	base := New(CodeDBError, "查询失败")
	inner := errors.New("connection refused")

	wrapped := base.Wrap(inner)

	if !errors.Is(wrapped, inner) {
		t.Error("Wrap 后应能通过 errors.Is 拿到底层错误")
	}
	if base.Err != nil {
		t.Error("Wrap 不应修改原错误（应返回副本）")
	}
	if wrapped.Error() == "" {
		t.Error("Error() 不应为空")
	}
}

// TestFromError 验证未知错误被归一化，不向客户端泄漏内部实现。
func TestFromError(t *testing.T) {
	if FromError(nil) != nil {
		t.Error("nil 应转成 nil")
	}

	// 已是业务错误的原样返回
	biz := New(CodeInvalidParams, "参数错误")
	if FromError(biz) != biz {
		t.Error("业务错误应原样返回，保留错误码")
	}

	// 普通错误必须降级为 500000，避免把内部细节抛给调用方
	unknown := FromError(errors.New("sql: no rows in result set"))
	if unknown.Code != CodeUnknown {
		t.Errorf("未知错误应为 CodeUnknown=%d，实际 %d", CodeUnknown, unknown.Code)
	}
	if unknown.Err == nil {
		t.Error("底层错误应被保留（用于写日志）")
	}
}

// TestShortcuts 验证快捷构造器生成的错误码。
func TestShortcuts(t *testing.T) {
	cases := []struct {
		name string
		err  *Error
		code int
	}{
		{"BadRequest", BadRequest("x"), CodeInvalidParams},
		{"Unauthorized", Unauthorized("x"), CodeInvalidToken},
		{"Forbidden", Forbidden("x"), CodeAccessDenied},
		{"NotFound", NotFound("x"), CodeNotFound},
		{"Internal", Internal("x"), CodeInternal},
		{"DB", DB("x", errors.New("e")), CodeDBError},
	}
	for _, c := range cases {
		if c.err.Code != c.code {
			t.Errorf("%s 错误码 = %d, 期望 %d", c.name, c.err.Code, c.code)
		}
	}
	// DB() 必须保留底层错误链，否则排障时看不到真实原因。
	if !errors.Is(DB("x", errSample), errSample) {
		t.Error("DB() 应保留底层错误链")
	}
}

// errSample 供测试使用的底层错误。
var errSample = errors.New("driver: bad connection")
