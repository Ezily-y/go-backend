// Package apperr 定义全站统一的业务错误码。
//
// 错误码分三段：HTTP 状态码 * 1000 + 序号。
// 例如 401001 = 401 未授权类的第 001 号错误。
// 这样做的好处：
//   - 从错误码即可反查 HTTP 状态，客户端不需要额外映射表；
//   - 同一状态下可细分多种业务原因，便于前端做差异化提示。
//
// 约定：code == 0 表示成功，非 0 一律是失败。
package apperr

import (
	"errors"
	"net/http"
)

// 通用成功/未知错误
const (
	CodeSuccess      = 0      // 成功
	CodeUnknown      = 500000 // 未知错误
	CodeBadRequest   = 400000 // 请求参数错误（兜底）
	CodeUnauthorized = 401000 // 未登录（兜底）
	CodeForbidden    = 403000 // 无权限（兜底）
	CodeNotFound     = 404000 // 资源不存在（兜底）
	CodeInternal     = 500000 // 服务器内部错误（兜底）
	CodeRateLimited  = 429000 // 触发限流
	CodeUnavailable  = 503000 // 服务暂时不可用
)

// 4xx 业务细分
const (
	CodeInvalidParams  = 400001 // 参数校验失败
	CodeInvalidToken   = 401001 // 令牌无效或已过期
	CodeBadCredentials = 401002 // 用户名或密码错误
	CodeAccessDenied   = 403001 // 缺少访问权限
	CodeUserExists     = 409001 // 用户名已存在
	CodeKeyExists      = 409002 // API Key 别名已存在
	CodeKeyInactive    = 401003 // API Key 已禁用
)

// 5xx 业务细分
const (
	CodeDBError      = 500001 // 数据库操作失败
	CodeExternalAPI  = 500002 // 第三方 API 调用失败
	CodeEncryptError = 500003 // 加解密失败
	CodeUploadFailed = 500004 // 文件上传失败
)

// Error 业务错误的统一载体，实现 error 接口。
// 所有需要返回给客户端的错误都应包装成 *Error，
// 中间件会据此生成统一的 JSON 响应。
type Error struct {
	Code    int    `json:"code"`    // 业务错误码
	Message string `json:"message"` // 面向用户的提示信息（可直接展示）
	HTTP    int    `json:"-"`       // 对应的 HTTP 状态码
	Err     error  `json:"-"`       // 原始错误，只写日志不返回给客户端
}

// Error 实现 error 接口。返回的是 Message 而非内部 Err，
// 避免把实现细节（SQL、堆栈）泄漏到日志以外的地方。
func (e *Error) Error() string {
	if e.Err != nil {
		return e.Message + ": " + e.Err.Error()
	}
	return e.Message
}

// Unwrap 保留错误链，让 errors.Is / errors.As 能穿透。
func (e *Error) Unwrap() error { return e.Err }

// HTTPStatus 返回该错误对应的 HTTP 状态码。
func (e *Error) HTTPStatus() int {
	if e.HTTP == 0 {
		return http.StatusInternalServerError
	}
	return e.HTTP
}

// Wrap 附加底层错误，返回新的 *Error（不修改原值，保证并发安全）。
func (e *Error) Wrap(err error) *Error {
	if err == nil {
		return e
	}
	clone := *e
	clone.Err = err
	return &clone
}

// New 构造一个不带底层错误的业务错误。
func New(code int, message string) *Error {
	return &Error{Code: code, Message: message, HTTP: httpFromCode(code)}
}

// NewWrap 构造一个带底层错误的业务错误。
func NewWrap(code int, message string, err error) *Error {
	return New(code, message).Wrap(err)
}

// ---- 常用快捷构造器 ----

// BadRequest 参数错误。
func BadRequest(msg string) *Error { return New(CodeInvalidParams, msg) }

// Unauthorized 未登录/令牌无效。
func Unauthorized(msg string) *Error { return New(CodeInvalidToken, msg) }

// Forbidden 无权限。
func Forbidden(msg string) *Error { return New(CodeAccessDenied, msg) }

// NotFound 资源不存在。
func NotFound(msg string) *Error { return New(CodeNotFound, msg) }

// Internal 服务器内部错误。
func Internal(msg string) *Error { return New(CodeInternal, msg) }

// DB 数据库错误。
func DB(msg string, err error) *Error { return NewWrap(CodeDBError, msg, err) }

// ---- 错误码 → HTTP 状态码 映射 ----

// httpFromCode 按错误码的千位前缀推导 HTTP 状态码。
// 规则：400001 → 400，401001 → 401，500001 → 500，即取码值除以 1000 的整数部分。
func httpFromCode(code int) int {
	if code == 0 {
		return http.StatusOK
	}
	prefix := code / 1000
	if prefix >= 400 && prefix <= 599 {
		return prefix
	}
	return http.StatusInternalServerError
}

// FromError 把任意 error 转换为 *Error：
//   - 已经是 *Error 的直接返回；
//   - 其余一律按未知错误处理，避免泄漏内部实现。
func FromError(err error) *Error {
	if err == nil {
		return nil
	}
	var e *Error
	if errors.As(err, &e) {
		return e
	}
	return NewWrap(CodeUnknown, "服务器内部错误", err)
}
