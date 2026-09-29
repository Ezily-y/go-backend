// Package docs 提供 API 文档：OpenAPI 3.1 spec + Scalar UI。
//
// 设计取舍（2026-09 重构，替换 swag 注解方案）：
//
//   - spec 用手写 Go struct 声明，而非注释驱动的代码生成。
//     原因：swag 只能产出 Swagger 2.0，且其类型解析依赖"注释所在文件的 import 表"，
//     属于隐式契约，容易在重构时静默失效。改为 struct 后由编译器保证，
//     且运行时直接 serve，无生成步骤、不依赖外部工具链。
//   - schema 复用 internal/model 与 internal/model/dto 的真实字段，
//     避免 doc 与实现漂移。
//   - UI 由 jsdelivr CDN 引入 Scalar，服务端只返回一页 HTML，
//     零打包、零 node 依赖；离线环境可改用自托管（见 ui.go 内的注释）。
//
// 生产环境（APP_MODE=release）下 router 会跳过注册，文档不会外泄。
package docs

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/getkin/kin-openapi/openapi3"
)

// 元信息，供 spec 与测试复用。
const (
	Title       = "go-backend API"
	Version     = "1.0"
	Description = "统一后端服务平台：第三方 API 数据请求与聚合、数据计算处理、前端渲染数据接口、数据持久化存储。"
)

// Spec 构建并校验完整的 OpenAPI 3.1 文档。
//
// 构建失败只可能来自内部结构错误（如 schema 引用不存在），
// 属于编程错误，直接 panic 让启动失败暴露问题，不静默降级。
func Spec() *openapi3.T {
	doc := build()

	// $ref 只写了 Ref 而 Value 为 nil，kin-openapi 的校验器会视为未解析。
	// 用 Loader 在内存里把所有 $ref 解析成实际 Value（不读外部文件）。
	var loader openapi3.Loader
	if err := loader.ResolveRefsIn(doc, nil); err != nil {
		panic(fmt.Sprintf("docs: 解析 $ref 失败: %v", err))
	}

	if err := doc.Validate(context.Background()); err != nil {
		panic(fmt.Sprintf("docs: 构建 OpenAPI spec 失败: %v", err))
	}
	return doc
}

// build 返回未经 ResolveRefsIn / Validate 的原始 doc，供诊断使用。
func build() *openapi3.T {
	return &openapi3.T{
		OpenAPI: "3.1.0",
		Info: &openapi3.Info{
			Title:       Title,
			Version:     Version,
			Description: Description,
		},
		Servers: openapi3.Servers{{URL: "/"}},
		Components: &openapi3.Components{
			Schemas:         schemas(),
			SecuritySchemes: securitySchemes(),
		},
		Paths:    paths(),
		Security: openapi3.SecurityRequirements{{"BearerAuth": nil}},
	}
}

// JSON 返回序列化后的 OpenAPI 3.1 JSON。
func JSON() []byte {
	b, err := json.Marshal(Spec())
	if err != nil {
		panic(fmt.Sprintf("docs: 序列化 OpenAPI spec 失败: %v", err))
	}
	return b
}

// SpecHandler 返回 serve OpenAPI 3.1 JSON 的处理器。
// 用 net/http.HandlerFunc 而非 gin.HandlerFunc，避免 docs 反向依赖 gin，
// 让本包可独立测试（无需构造 gin.Engine）。
func SpecHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(JSON())
	}
}
