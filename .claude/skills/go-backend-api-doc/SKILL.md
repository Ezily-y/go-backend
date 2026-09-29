---
name: go-backend-api-doc
description: 在 go-backend 项目里为新增/变更的 API 补 OpenAPI 3.1 文档。当用户要求"补文档""加接口文档""docs/paths.go 写一条""接口没在 /docs 里显示"或 go test 报 docscheck 漂移时使用。
---

# go-backend API 文档维护

本项目的 API 文档**不是生成的，是手写的** —— spec 用 Go struct 声明在 `docs/` 包里，
运行时直接序列化成 OpenAPI 3.1 返回给 Scalar UI。**没有 swag、没有注解、没有生成步骤。**

代价是：新增接口时 `router.go` 和 `docs/paths.go` 是两份独立事实源，容易漏写。
防漂移测试 `internal/docscheck` 会把漏写变成 `go test` 直接失败 —— 见第 0 节。

## 0. 先跑测试，让它告诉你缺什么

```bash
go test ./internal/docscheck/ -v
```

失败信息会精确列出缺失或过期的 `METHOD path`，以及修复位置。**照着它做即可，不要靠记忆补。**

双向校验的含义：

| 失败方向 | 含义 | 修复 |
|---|---|---|
| 「路由已注册但 spec 缺失」 | 新接口没写文档 | 往 `docs/paths.go` 加条目 |
| 「spec 条目在路由中不存在」 | 接口删了/路径写错 | 从 `docs/paths.go` 删或改 |

`/health` `/ready` `/metrics` `/docs` `/openapi.json` `/uploads/*` 在 `skipRoutes` 里豁免
（基础设施端点，spec 里可以写但不参与比对）。

## 1. 需要改哪些文件

| 场景 | 改动 |
|---|---|
| 新增/删除/改路径、改方法 | `docs/paths.go` |
| 请求/响应出现新结构 | `docs/schemas.go`（`schemas()` 加键 + 一个 `xxxSchema()` 函数） |
| 新增鉴权方式 | `docs/schemas.go` 的 `securitySchemes()` + `docs/helpers.go` 加 `withXxx()` |
| 只改字段说明、示例 | `docs/schemas.go` 对应 schema 函数 |
| **改了 router 却没动 docs** | 测试会失败 —— 这是设计如此 |

`docs/registry()` 从 `schemas()` 自动派生，**新增 schema 只需在 `schemas()` 加一行**，
不用改 registry。

## 2. 写一条 path（模板）

照抄现有条目的结构。以「创建文章」为例，加在 `docs/paths.go` 的 `return p` 之前：

```go
// ---- 文章 ----
p.Set("/api/v1/articles", &openapi3.PathItem{})
arts := p.Find("/api/v1/articles")

arts.Get = withBearer(op("文章列表", "文章"))
arts.Get.Parameters = append(pageParams(),
    queryStr("keyword", "模糊匹配标题", 64),
)
setStatus(arts.Get, 200, jsonOK("分页文章列表", "ArticlePage"))
setStatus(arts.Get, 401, errResp("未登录或令牌无效"))
setStatus(arts.Get, 403, errResp("无权访问"))

arts.Post = withBearer(op("创建文章", "文章"))
arts.Post.RequestBody = bodyReq("文章信息", "CreateArticleRequest")
setStatus(arts.Post, 200, jsonOK("创建成功", "Article"))
setStatus(arts.Post, 400, errResp("参数校验失败"))
setStatus(arts.Post, 401, errResp("未登录或令牌无效"))
setStatus(arts.Post, 409, errResp("标题已存在"))
```

要点：

- **`op(summary, tag)` 的两个参数必填** —— tag 决定 Scalar UI 的分组，沿用已有 tag
  （`认证` / `用户管理` / `API Key` / `系统` / `外部工具`），新资源才建新 tag。
- **鉴权二选一**：`withBearer(...)` 走 JWT（`Authorization: Bearer`），
  `withApiKey(...)` 走 API Key（`X-API-Key`）。与 `router.go` 里挂的中间件必须一致。
- `op()` 自带 400 / 500 默认响应，**只补 200 和该接口特有的错误码**。
- 路径参数用 `{id}`（OpenAPI），不是 gin 的 `:id` —— 漂移测试会做转换后比对。
- 分页接口用 `pageParams()`，不要手写 page/page_size。

### helper 速查

| helper | 用途 |
|---|---|
| `jsonOK(desc, "SchemaName")` | 200 + `dataBody(refSchema(...))`，最常用 |
| `jsonOKEmpty(desc)` | 200 但无业务数据（data 为 `{}`） |
| `errResp(desc)` | 某个状态码的错误响应 |
| `bodyReq(desc, "SchemaName")` | JSON 请求体 |
| `pageParams()` | page / page_size 查询参数 |
| `pathID(desc)` | `:id` 路径参数 |
| `queryStr(name, desc, maxLen)` / `queryEnum(name, desc, v...)` | 查询参数 |
| `setStatus(op, code, resp)` | 挂响应（覆盖同码默认值） |

## 3. 写一个 schema（模板）

`docs/schemas.go` 里加函数，并在 `schemas()` 的 map 中登记：

```go
// schemas() 里加一行（按分组放）
"Article": articleSchema(),

// 文件末尾定义
func articleSchema() *openapi3.SchemaRef {
    return prop(withRequired(withProps(desc(obj(), "文章"), map[string]*openapi3.SchemaRef{
        "id":         prop(desc(integer(), "文章 ID")),
        "title":      prop(ex(str(), "标题")),
        "author":     ref("User"),                          // 引用其他 schema
        "created_at": prop(withFormat(str(), "date-time")),
    }), "id", "title"))
}
```

要点：

- 字段与 `internal/model` / `internal/model/dto` 的**真实字段一一对应**，
  敏感字段（`json:"-"` 的）**不要写进 schema**。
- `prop(...)` 包裸 schema；引用已有 schema 用 `ref("Name")`。
- 可空字段用 `nullable(...)`（3.1 下产出 `["string","null"]`）。
- 链式修饰：`desc` `ex` `enum` `withFormat` `minLen` `maxLen` `minMax`。
- **不要给 `openapi3.Schema` / `openapi3.Operation` 加方法** —— Go 禁止给外部类型定义方法
  （报 `cannot define new methods on non-local type`），一律用包内函数。

## 4. 分页响应

列表接口的 `data` 形状固定为 `{list, total, page, page_size}`（见 `response.Page`）。
用 `pageBody("Entity")` 生成：

```go
"ArticlePage": prop(pageBody("Article")),
```

## 5. 提交前必跑

```bash
gofmt -l .                       # 必须无输出
go build ./...
go vet ./...
go test ./...                    # 含 docscheck 漂移门禁
```

`gofmt` / `go vet` / `go test` 已在 `.claude/settings.json` 放行。

## 6. 常见报错

| 报错 | 原因 | 修复 |
|---|---|---|
| `cannot define new methods on non-local type openapi3.Xxx` | 给库类型加了方法 | 改成包内函数 |
| `found unresolved ref: "#/components/schemas/X"` | schema 没在 `schemas()` 登记，或 ref 名拼错 | 在 `schemas()` 加键 / 核对拼写 |
| `invalid default: ... invalid jsonType *int` | `Default` 赋了指针 | 改成标量 `Default = 1` |
| `value MUST be an object` | 某个 `SchemaRef` 的 `Ref` 和 `Value` 都为空 | 检查是否漏了 `prop()` |
| drift test 报 missing | 路由加了、文档没加 | 按第 2 节补 `paths.go` |
| drift test 报 stale | 文档写了不存在的路由 | 从 `paths.go` 删除或改路径 |

## 7. 查看效果

```bash
make run
# 浏览器打开
#   http://localhost:8080/docs            Scalar UI
#   http://localhost:8080/openapi.json    原始 spec（可导入 Apifox）
```

`APP_MODE=release` 时这两条路由不注册。
