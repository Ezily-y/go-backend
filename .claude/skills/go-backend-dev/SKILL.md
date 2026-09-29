---
name: go-backend-dev
description: 在 go-backend 项目里新增 API 接口、加配置项、写测试。当用户要求"加一个接口""新增路由""加配置""写个 handler"时使用。
---

# go-backend 开发流程

## 0. 动手前先读（不要凭记忆写）

项目结构已稳定，但仍建议动手前快速确认现状：

```bash
find internal cmd -type f -name '*.go'
```

必读（都是已落地的文件）：

| 文件 | 为什么 |
|---|---|
| `internal/response/response.go` | 统一响应格式，**所有接口必须走它** |
| `internal/apperr/apperr.go` | 错误码表 + `*Error` 构造器 |
| `internal/model/dto/dto.go` | 请求/响应 DTO 的写法与分页约定 |
| `internal/model/model.go` | 实体、`Role`、`TableName()` |
| `internal/config/config.go` | 配置结构 / defaults / bindEnvs |
| `internal/database/database.go` | `Get()`、`Migrate()`、`Ping()` |

## 1. 统一响应格式（硬约束）

```go
{"code": 0, "message": "success", "data": {}}
```

```go
// 成功
response.OK(c, user)
response.OKWithMessage(c, "创建成功", user)
response.Page(c, list, total, q.Page, q.PageSize)   // 分页

// 失败 —— 只传 error，不要自己拼 JSON
response.Fail(c, apperr.BadRequest("username 不能为空"))
response.Fail(c, apperr.DB("查询用户失败", err))
response.FailWithCode(c, apperr.CodeUserExists, "用户名已存在")
```

- `data` 为 nil 时自动变 `{}`，不会发 `null`。
- **5xx 对外只给 `"服务器内部错误"`**，真实原因写日志：`logger.Errorf("...: %v", err)`。
- 禁止 `c.JSON(http.StatusOK, gin.H{...})` 绕过本包 —— 前端按 `code` 分支判断。

## 2. 新增一个接口的完整步骤

### 2.1 DTO（`internal/model/dto/dto.go`）

请求/响应结构体与持久化实体**分开**，避免数据库字段变更直接改 API 契约。

```go
// CreateArticleRequest 创建文章请求。
type CreateArticleRequest struct {
    Title string `json:"title" binding:"required,min=2,max=200" example:"标题"`
    Body  string `json:"body"  binding:"required" example:"正文"`
}

// ListArticleRequest 文章列表查询。
type ListArticleRequest struct {
    PageQuery                                   // 内嵌复用分页
    Keyword string `form:"keyword" json:"keyword" binding:"max=64"`
}
```

- 校验用 `binding` tag（Gin 的 validator）；取值用 `form`（query）或 `json`（body）。
- 列表请求**内嵌 `PageQuery`**，进 handler 先 `q.Normalize()` 再 `q.Offset()`。
- 响应体的敏感字段一律 `json:"-"`。

### 2.2 实体（`internal/model/model.go`）—— 仅新增表时

```go
type Article struct {
    ID        uint      `json:"id" gorm:"primaryKey;autoIncrement"`
    Title     string    `json:"title" gorm:"size:200;not null"`
    CreatedAt time.Time `json:"created_at" gorm:"not null"`
}

func (Article) TableName() string { return "articles" }   // 必须，避免 GORM 复数化
```

**然后必须把 `&model.Article{}` 加进 `internal/database/database.go` 的 `Migrate()` `targets` 切片**，否则表不会建、运行时报 no such table。

### 2.3 Handler

签名统一为 `func Xxx(c *gin.Context)`。Handler 文件位于 `internal/handler/`，当前已有 `auth.go`、`user.go`、`system.go`。新增接口按资源归类到对应文件（如新加文章相关接口则创建 `internal/handler/article.go`）。

骨架：

```go
// Create 创建文章。
func Create(c *gin.Context) {
    var req dto.CreateArticleRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        response.Fail(c, apperr.BadRequest("参数校验失败: "+err.Error()))
        return
    }

    art := &model.Article{Title: req.Title}
    if err := database.Get().Create(art).Error; err != nil {
        logger.Errorf("创建文章失败: %v", err)
        response.Fail(c, apperr.DB("创建文章失败", err))
        return
    }

    response.OK(c, art)
}
```

要点：
- `ShouldBindJSON` 失败 → `apperr.BadRequest`，**不要**把 validator 的原始长串直接吐给前端时省掉上下文。
- 数据库错误先 `logger.Errorf` 记全量，再 `apperr.DB` 返回。
- 查询用 `errors.Is(err, gorm.ErrRecordNotFound)` 判空 → `apperr.NotFound("文章不存在")`。

### 2.4 路由注册

路由注册文件位于 `internal/router/router.go`。按资源分组注册（例：`/api/v1` 下按资源分组）：

```go
art := api.Group("/articles")
{
    art.GET("", List)
    art.GET("/:id", Get)
    art.POST("", Create)
}
```

- 需要鉴权的接口挂到已有的 auth 中间件分组里，**不要自己再造一个中间件**。
- 需要管理员权限的确认是否已有角色中间件可复用（`model.Role` admin/editor/viewer）。
- **注册完路由就要补文档**（见 2.6）：`docs/paths.go` 加对应条目，
  鉴权方式要与这里挂的中间件一致（JWT → `withBearer`，API Key → `withApiKey`）。

### 2.5 测试

测试文件与被测包同目录，命名 `xxx_test.go`：

```go
func TestCreateArticle(t *testing.T) {
    w := httptest.NewRecorder()
    c, _ := gin.CreateTestContext(w)
    // ... 触发 handler，断言统一响应格式
    var body struct {
        Code    int         `json:"code"`
        Message string      `json:"message"`
        Data    interface{} `json:"data"`
    }
    _ = json.Unmarshal(w.Body.Bytes(), &body)
    if body.Code != apperr.CodeSuccess {
        t.Fatalf("期望 code=0，实际 %d", body.Code)
    }
}
```

**断言统一响应体的 `code` 字段，不要只断言 HTTP 200** —— 业务错误也可能是 200/4xx，真正契约在 `code`。

### 2.6 API 文档（**必做，否则 go test 直接失败**）

本项目 API 文档是**手写**的 OpenAPI 3.1 spec，位于 `docs/` 包，运行时 serve 到
`/docs`（Scalar UI）与 `/openapi.json`。**没有 swag、没有注解、没有生成步骤。**

新增/删除/改路径的接口，必须同步改 `docs/paths.go`（涉及新结构再加 `docs/schemas.go`）。

`internal/docscheck` 的漂移测试会双向比对「实际注册的路由」与「spec 条目」：
漏写或写多都会让 `go test ./...` 失败。

**完整写法与模板见 `.claude/skills/go-backend-api-doc/SKILL.md`** —— 动手前先跑：

```bash
go test ./internal/docscheck/ -v    # 失败信息会精确指出缺哪条
```

### 2.7 提交前必跑

```bash
gofmt -w internal/ cmd/     # 就地格式化
gofmt -l .                  # 必须无输出
go vet ./...
go test ./...               # 含 docscheck 文档漂移门禁
```

（`gofmt` / `go vet` / `go test` 已在 `.claude/settings.json` 放行。）

注意：改完代码若新增或更换了依赖，需要整理依赖：

```bash
go mod tidy
```

## 3. 新增配置项（四处同步，缺一不生效）

以新增 `app.max_upload_size` 为例，**必须同时改**：

1. **结构体** `internal/config/config.go` → `AppConfig`：
   ```go
   MaxUploadSize int64 `mapstructure:"max_upload_size"`
   ```
2. **默认值** → `setDefaults()`：
   ```go
   v.SetDefault("app.max_upload_size", 10<<20)
   ```
3. **环境变量绑定** → `bindEnvs()` 的 `pairs`：
   ```go
   {"APP_MAX_UPLOAD_SIZE", "app.max_upload_size"},
   ```
4. **配置文件** `config/config.yaml`：
   ```yaml
   app:
     max_upload_size: 10485760   # 单次上传上限（字节）
   ```

若该值有合法性要求，同时在 `Config.Validate()` 里加校验（现有范式：返回 `fmt.Errorf("配置非法: ...")`，启动即失败）。

**敏感值不要写进 `config/config.yaml`**，只走 `bindEnvs` + 环境变量（参照 `JWT_SECRET`）。
列表型环境变量要仿照 `APP_CORS_ORIGINS` 在 `bindEnvs` 里手工 `strings.Split`。

## 4. 日志

```go
logger.Infof("创建文章: id=%d", art.ID)
logger.Warnf("慢查询: table=%s cost=%s", t, d)
logger.Errorf("创建文章失败: %v", err)     // error 级别自带堆栈
logger.Zap().Info("request", zap.String("path", c.Request.URL.Path))
```

- 格式化日志用 `Infof/Errorf` 这类包级函数；需要结构化字段才用 `logger.Zap()`。
- 不用 `fmt.Println` / `panic` 代替日志；`panic` 只在不可恢复且启动期的场景。
