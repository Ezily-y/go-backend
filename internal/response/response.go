// Package response 提供全站统一的 JSON 响应格式。
//
// 所有接口一律返回：
//
//	{"code": 0, "message": "success", "data": {}}
//
// code == 0 表示成功；非 0 时 data 通常为 null，
// 具体错误含义见 internal/apperr 的错误码表。
package response

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"go-backend/internal/apperr"
)

// Body 统一响应体。
type Body struct {
	Code    int         `json:"code"`    // 业务错误码，0 表示成功
	Message string      `json:"message"` // 提示信息
	Data    interface{} `json:"data"`    // 业务数据，失败时为 null
}

// OK 返回成功响应。
func OK(c *gin.Context, data interface{}) {
	if data == nil {
		data = map[string]struct{}{} // 避免序列化成 "data": null
	}
	c.JSON(http.StatusOK, Body{Code: apperr.CodeSuccess, Message: "success", Data: data})
}

// OKWithMessage 返回带自定义成功提示的响应。
func OKWithMessage(c *gin.Context, msg string, data interface{}) {
	if data == nil {
		data = map[string]struct{}{}
	}
	c.JSON(http.StatusOK, Body{Code: apperr.CodeSuccess, Message: msg, Data: data})
}

// Page 返回分页数据，保持外层结构不变。
// data 形如 {"list": [...], "total": 100, "page": 1, "page_size": 20}
func Page(c *gin.Context, list interface{}, total int64, page, pageSize int) {
	OK(c, gin.H{
		"list":      list,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// Fail 返回失败响应。
// 传入的 err 会被归一化为 *apperr.Error：
// 业务错误保留其错误码，未知错误统一降级为 500000，避免泄漏内部细节。
func Fail(c *gin.Context, err error) {
	e := apperr.FromError(err)
	status := e.HTTPStatus()

	// 客户端侧错误才把 Message 原样透出；
	// 服务端 5xx 对外只给通用文案，真实原因写在服务端日志里。
	msg := e.Message
	if status >= 500 {
		msg = "服务器内部错误"
	}
	c.JSON(status, Body{Code: e.Code, Message: msg, Data: nil})
}

// FailWithCode 返回指定错误码的失败响应（无底层错误）。
func FailWithCode(c *gin.Context, code int, msg string) {
	Fail(c, apperr.New(code, msg))
}
