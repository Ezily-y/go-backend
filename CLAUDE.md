# go-backend 项目指引

> 本文档只写已经过代码验证的内容。动手前先用 Glob/Grep 重新确认关键路径，不要按本文档的猜测硬套。

## 项目是什么

Go 1.26 + Gin 的后端服务，module 名 `go-backend`，默认监听 `:8080`（`config/app.port`，可用环境变量 `APP_PORT` 覆盖）。
持久化用 GORM（默认 sqlite `./data/app.db`，可切 postgres），配置用 viper，日志用 zap。
GitHub Actions 构建 Docker 镜像推腾讯云 TCR，再 SSH 到腾讯云服务器 `docker compose` 部署。

## 目录结构

已存在的（**逐一验证过**）：

| 路径 | 职责 |
|---|---|
| `cmd/server/main.go` | 程序入口 `main()`（`cmd/api/` 是空目录，不参与构建） |
| `internal/config/config.go` | viper 配置加载：`setDefaults` → `config/config.yaml` → `bindEnvs`；`Load(path)`、`Validate()`、`Addr()` |
| `internal/logger/logger.go` | zap 全局日志。`Initialize(Options{Level,Format,File})`，包级 `Debugf/Infof/Warnf/Errorf/Fatalf`，`Zap()` 取原始句柄，`Sync()` 退出时调用 |
| `internal/response/response.go` | 统一 JSON 响应。`OK` / `OKWithMessage` / `Page` / `Fail` / `FailWithCode` |
| `internal/apperr/apperr.go` | 业务错误码与 `*Error`。`New` / `NewWrap` / `BadRequest` / `Unauthorized` / `Forbidden` / `NotFound` / `Internal` / `DB` / `FromError` |
| `internal/database/database.go` | `Connect(cfg)` 建连 + `Migrate()` + `Ping(ctx)` + `Close()`；`Get()` 返回全局 `*gorm.DB` |
| `internal/model/model.go` | 实体：`User` / `APIKey` / `APILog`，`Role`（admin/editor/viewer），`NewID(n)` |
| `internal/model/dto/dto.go` | 请求/响应 DTO：登录、用户、API Key、`PageQuery`（`Normalize()`/`Offset()`） |
| `internal/auth/` | JWT 签发与校验（`jwt.go`）、密码哈希（`crypto.go`），含单元测试 |
| `internal/router/router.go` | 路由注册与中间件挂载。全局中间件顺序：RequestID → Recovery → Logger → CORS → Metrics |
| `internal/middleware/` | `auth.go`（JWT + API Key + RequireAdmin）、`middleware.go`（RequestID/Recover/Logger/CORS）、`ratelimit.go`（令牌桶限流） |
| `internal/handler/` | HTTP handler 层：`auth.go`（AuthHandler）、`user.go`（UserHandler + APIKeyHandler）、`system.go`（SystemHandler） |
| `internal/service/` | 业务逻辑层：`user.go`、`apikey.go` |
| `internal/repository/` | 数据访问层：`repository.go`（基础封装）、`user.go`、`apikey.go` |
| `internal/bootstrap/bootstrap.go` | 首启自动创建管理员账号 |
| `internal/metrics/metrics.go` | Prometheus 指标定义与 HTTP handler |
| `internal/version/version.go` | 构建期通过 `-ldflags` 注入的版本/commit/时间信息 |
| `internal/docscheck/drift_test.go` | 文档与代码一致性检查 |
| `docs/` | OpenAPI spec 与 Scalar UI（运行时构造，无生成步骤），含 `smoke_test.go` |
| `config/config.yaml` | 非敏感配置文件 |

目录已建但**当前为空**：

| 路径 | 预期职责 |
|---|---|
| `scripts/` | 辅助脚本 |

`go.mod` 当前有 14 条直接依赖：gin、gorm、viper、zap、jwt/v5、uuid、prometheus/client_golang、kin-openapi、glebarez/sqlite、golang.org/x/crypto、golang.org/x/time、lumberjack、gorm/driver/postgres。

## 常用命令

```bash
go build ./...            # 编译
go vet ./...              # 静态检查
go test ./...             # 跑测试（不能用 -race，本机 windows/386 不支持且需要 CGO）
gofmt -w internal/ cmd/   # 格式化（就地改文件）
gofmt -l .                # 只列出未格式化的文件（CI 应用这个当门禁）
go run ./cmd/server       # 启动服务
go mod tidy               # 整理依赖
make help                 # 查看 Makefile 全部目标（需安装 make）
```

- 配置文件默认读 `config/config.yaml`（相对 cwd），可用 `CONFIG_PATH` 环境变量换路径，也可用 `-c` 标志指定。
- 环境变量覆盖配置（键名固定，见 `internal/config/config.go` 的 `bindEnvs`）：
  `APP_NAME` `APP_MODE` `APP_PORT` `APP_UPLOAD_DIR` `APP_CORS_ORIGINS`（逗号分隔）
  `JWT_SECRET` `JWT_ACCESS_TTL` `JWT_REFRESH_TTL`
  `LOG_LEVEL` `LOG_FORMAT` `LOG_FILE`
  `DB_DRIVER` `DB_DSN`
  `REDIS_ENABLED` `REDIS_ADDR` `REDIS_PASSWORD`
- 本地起服务示例：
  ```bash
  APP_MODE=debug JWT_SECRET="$(head -c 48 /dev/urandom | base64 | tr -d '\n')" go run ./cmd/server
  ```
- 本机工具链：go 1.26.0（windows/386） / git 2.47 / gh 2.65；**golangci-lint 未安装**，**make 未安装**。`Makefile` 供 CI 与装了 make 的机器用，本地直接敲原始 go 命令。docker daemon 默认未启动。
- 仓库根有 `.golangci.yml` 时（当前已有，**v2** schema，与 v1 不兼容），本地跑 `golangci-lint run` 与 CI 用同一套规则。
- API 文档路由：`/docs`（Scalar UI）、`/openapi.json`（spec）。是否注册由 **`app.docs_enabled`** 决定，与 `app.mode` 解耦：本地默认开；生产 `compose.prod.yaml` 传 `DOCS_ENABLED=false` 关闭，需要时用环境变量单独打开（不要为此把 mode 退回 debug）。

## 架构约定

### 配置加载顺序

`setDefaults(v)` → `v.ReadInConfig()` 读 `config/config.yaml` → `bindEnvs(v)` → `v.Unmarshal` → `cfg.Validate()`。
后者覆盖前者，所以**环境变量优先级最高**。`Validate()` 不通过直接返回 error，启动即失败。

新增配置项必须**同时改四处**，缺一处就是"配置不生效"：
1. `internal/config/config.go` 的 `Config` 结构体（`mapstructure` 标签）
2. 同文件 `setDefaults()` 里的 `v.SetDefault(...)`
3. 同文件 `bindEnvs()` 里的 `[2]string{"ENV_NAME", "config.key"}`
4. `config/config.yaml` 加同名键（带注释）

`Validate()` 里现有硬约束：`app.port ∈ [1,65535]`、`app.mode ∈ {debug,release,test}`、
**`app.jwt.secret` 长度必须 >= 32**、`refresh_ttl >= access_ttl`、
`database.driver ∈ {sqlite,postgres}`、`log.level ∈ {debug,info,warn,error}`。

### 统一响应格式（`internal/response`）

```json
{"code": 0, "message": "success", "data": {}}
```

- 成功：`response.OK(c, data)`；`data == nil` 会被转成 `{}`，不发 `null`。
- 分页：`response.Page(c, list, total, page, pageSize)` → `data` 形如 `{"list":[...],"total":100,"page":1,"page_size":20}`。
- 失败：`response.Fail(c, err)`，内部走 `apperr.FromError(err)` 归一化。
  **5xx 只对外返回 `"服务器内部错误"`**，真实原因必须写进服务端日志，不要自己 `c.JSON` 拼错误。
- 禁止绕过本包直接 `c.JSON(http.StatusOK, gin.H{...})` —— 前端按 `code` 字段分支。

### 错误处理（`internal/apperr`）

错误码 = `HTTP状态码 * 1000 + 序号`，`code/1000` 即 HTTP 状态（`httpFromCode`）。
`code == 0` 成功，非 0 一律失败。

| 常量 | 值 | 含义 |
|---|---|---|
| `CodeSuccess` | 0 | 成功 |
| `CodeInvalidParams` | 400001 | 参数校验失败 |
| `CodeInvalidToken` | 401001 | 令牌无效/过期 |
| `CodeBadCredentials` | 401002 | 账号或密码错误 |
| `CodeKeyInactive` | 401003 | API Key 已禁用 |
| `CodeAccessDenied` | 403001 | 无权限 |
| `CodeUserExists` | 409001 | 用户名已存在 |
| `CodeKeyExists` | 409002 | API Key 别名已存在 |
| `CodeDBError` | 500001 | 数据库操作失败 |
| `CodeInternal` | 500000 | 服务器内部错误（兜底） |

写法约定：

```go
// 业务错误：构造时就带上底层 err，日志里保留链路，对外只给 Message
return apperr.DB("查询用户失败", err)
return apperr.BadRequest("username 不能为空")

// 上层不要再包一层，response.Fail 会自动归一化
if err := repo.Save(u); err != nil {
    return apperr.DB("保存用户失败", err)
}
```

`*Error` 实现了 `Unwrap()`，`errors.Is` / `errors.As` 能穿透。
未知错误一律降级为 `CodeUnknown(500000)`，避免泄漏 SQL/堆栈。

### 日志（`internal/logger`）

```go
logger.Infof("用户登录: user=%s ip=%s", name, ip)   // 常规
logger.Warnf("慢查询: table=%s cost=%s", t, d)       // 可恢复异常
logger.Errorf("下单失败: order=%s err=%v", id, err)  // error 级别自带堆栈
logger.Zap().Info("request", zap.String("path", "/api/v1/users"))  // 强类型字段
```

- 格式 `console`（本地）/ `json`（生产，供采集系统解析），级别 `debug|info|warn|error`。
- `log.file` 非空时同时写文件，lumberjack 滚动：100MB / 留 7 份 / 留 30 天 / gzip。
- 启动时必须调一次 `logger.Initialize(...)`，退出时 `defer logger.Sync()`。
- 不要在业务代码里 `fmt.Println`。

### 数据库（`internal/database`）

- `database.Connect(cfg)` 完成建连 + 连通性 Ping + `Migrate()`，失败直接返回 error，调用方应当 `Fatalf`。
- `database.Get()` 拿 `*gorm.DB`；**未初始化时会 panic**，只能在 `Connect` 之后调用。
- sqlite 参数已固定：WAL + `busy_timeout=5000` + 外键开启 + `synchronous=NORMAL`。
- SQLite 驱动走纯 Go（`github.com/glebarez/sqlite`），全程 `CGO_ENABLED=0`。
- 迁移用 `AutoMigrate`，当前目标：`model.User` / `model.APIKey` / `model.APILog`。**新增实体必须加进 `database.Migrate()` 的 `targets`**，否则表不会建。
- SQL 日志桥到 zap，慢查询阈值 `database.slow_threshold`（默认 200ms）。

### 路由与中间件（`internal/router`）

- 全局中间件顺序（顺序敏感）：RequestID → Recovery → Logger → CORS → Metrics。
- 鉴权按路由组分别挂载：公开区（`/health`、`/ready`、`/metrics`、`/uploads`）/ JWT 区（`/api/v1` 大部分）/ API Key 区（`/api/v1/tool`）。
- 登录接口挂限流（`middleware.RateLimiter`），注意限流中间件必须写在 handler 之前。
- 健康检查路径是 **`/health`**，不是 `/healthz`（后者 404）。
- Prometheus 指标路径：`/metrics`。
- 404 / 405 统一用 `response.Fail` / `response.FailWithCode` 返回标准响应体。

### 模型与 DTO

- 实体放 `internal/model`，带 `db`/`gorm` 标签，实现 `TableName()`（避免 GORM 复数化）。
- 对外结构体放 `internal/model/dto`，只带 `json` + `binding` 标签。**两层分离，不要让数据库字段变更直接改 API 契约。**
- 密码/密钥字段一律 `json:"-"`（见 `User.PasswordHash`、`APIKey.KeyHash`），文档 schema 由 `docs/schemas.go` 手工排除。
- `PasswordHash` 存哈希，不存明文；API Key 明文只在创建时返回一次，库里只存 `KeyPrefix` + `KeyHash`。
- 时间统一 UTC。
- 分页请求复用 `dto.PageQuery`，先调 `Normalize()` 再算 `Offset()`。

### JWT / 安全

- HS256，密钥来自 `app.jwt.secret`，**长度 < 32 直接启动失败**。
- 默认值 `dev-only-secret-change-me-0123456789` **只用于本地**，生产必须用 `JWT_SECRET` 覆盖。
- `app.bootstrap` 首启建管理员账号，默认 `admin / admin123456`，生产必须覆盖或关闭 `app.bootstrap.enabled`。

## 部署

链路：`push main` → `.github/workflows/ci.yml`（gofmt + vet + golangci-lint + build + test）→
`.github/workflows/deploy.yml`（复用 CI 做质量门禁 → 构建镜像推腾讯云 TCR → SSH 到服务器 → `docker compose up -d`）。

| 文件 | 作用 |
|---|---|
| `.github/workflows/ci.yml` | 门禁。同时支持 `push`/`pull_request` 和 `workflow_call`（被 deploy 复用）。Go 版本从 `go.mod` 读取，不写死 |
| `.github/workflows/deploy.yml` | 构建镜像 → 推 TCR → scp 下发 compose → SSH 部署 |
| `Dockerfile` | 多阶段构建，**builder 必须 `golang:1.26-alpine`**（`go.mod` 是 `go 1.26.0`，用 1.25 会直接失败） |
| `compose.prod.yaml` | **生产**编排，从 TCR 拉镜像。文件名不在 Docker Compose 自动发现列表，必须显式 `-f` |
| `docker-compose.yaml` | **开发**编排（postgres + redis）。`docker compose up` 默认用这个 |
| `deploy/server-setup.sh` | 服务器一次性初始化（Docker + 镜像加速 + 目录 + .env） |
| `.golangci.yml` | golangci-lint **v2** 配置，v1 的 schema 不兼容，勿照抄 v1 示例 |

**需要的 GitHub Secrets**（完整清单与排查见 `.claude/skills/go-backend-deploy/SKILL.md`）：
`TCR_REGISTRY` `TCR_IMAGE` `TCR_USERNAME` `TCR_PASSWORD`
`DEPLOY_HOST` `DEPLOY_USER` `DEPLOY_SSH_KEY` `DEPLOY_PATH`

### 已实测确认的事实（改动前先看，别再踩一遍）

- **健康检查路径是 `/health`，不是 `/healthz`**。实测 `/healthz` 返回 404；
  真实路由见 `internal/router/router.go`：`e.GET("/health", ...)`、`/ready`、`/metrics`。
- **入口是 `./cmd/server`**。`cmd/api/` 是空目录，不参与构建。
- **SQLite 走纯 Go**：`github.com/glebarez/sqlite`，所以全程 `CGO_ENABLED=0`，
  交叉编译 `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build ./cmd/server` 可产出静态 ELF。
- **`-race` 用不了**：本机 `windows/386` 报 `-race is not supported`，且 `-race` 需要 CGO。
- **本机没装 `make` 和 `golangci-lint`**，`Makefile` 供 CI 与装了 make 的机器用；本地直接敲原始 go 命令。
- `golangci-lint` 装在 `$(go env GOPATH)/bin`（不在 PATH），全量检查当前有 13 条待处理问题。
- **构建期注入版本信息**：通过 `-ldflags` 写入 `internal/version` 包（`Version`/`Commit`/`BuildTime`），`Makefile` 与 `Dockerfile` 均已配置。

**回滚**：改服务器 `${DEPLOY_PATH}/.env` 里的 `IMAGE` 为上一个 `sha-*` 标签，再 `docker compose -f compose.prod.yaml up -d`。

作者 GitHub 账号：`Ezily-y`。仓库当前**没有任何 commit**（`main` 分支尚无提交），首次提交前先确认 `.gitignore` 覆盖到位。

## 本次会话注意事项

1. **敏感信息只走环境变量**，不写进 `config/config.yaml`、不写进代码、不写进任何会提交的文件。
2. `JWT_SECRET` 长度 **>= 32**，否则 `Config.Validate()` 直接拒绝启动。
3. **不要提交 `.env`**（已在 `.gitignore`），也不要创建会含真实密钥的 `config/config.local.yaml` 之外的配置副本。
4. `.claude/settings.json` 里**禁止出现任何 API key / token / 密码**（`env` 字段整体别写密钥）。
5. （原为"只改 `.claude/` 和 `CLAUDE.md`"，适用于之前与其他进程并行写入的会话，当前已无此限制。）
6. 提交前跑：`gofmt -l .`（应无输出）→ `go vet ./...` → `go test ./...`。
7. 本机是 Windows 11 + Git Bash，shell 命令按 POSIX 写（`/dev/null` 不是 `NUL`，正斜杠路径）。
