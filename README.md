# go-backend

> 统一后端服务平台 —— 第三方 API 数据请求与聚合、数据计算处理、前端渲染数据接口、数据持久化存储。

**Go 1.26.0** · Gin · GORM · Viper · Zap · Prometheus · JWT

---

## 目录

- [项目概述](#项目概述)
- [技术栈](#技术栈)
- [快速开始](#快速开始)
- [Docker Compose 开发环境](#docker-compose-开发环境)
- [常用命令](#常用命令)
- [目录结构](#目录结构)
- [请求处理流程](#请求处理流程)
- [统一响应格式](#统一响应格式)
- [错误码体系](#错误码体系)
- [API 接口一览](#api-接口一览)
- [认证机制](#认证机制)
- [数据库](#数据库)
- [配置管理](#配置管理)
- [部署](#部署)
- [常见问题排查](#常见问题排查)

---

## 项目概述

| 模块 | 状态 | 说明 |
|---|---|---|
| 项目骨架 | ✅ | 分层架构 + 统一响应 + 统一错误码 + 配置外置 |
| 用户与认证 | ✅ | JWT 登录 / 刷新 / 改密，RBAC 三角色，API Key 管理与校验 |
| 数据存储 | ✅ | GORM 通用 CRUD、自动迁移，SQLite 默认 / PostgreSQL 可切 |
| 系统管理 | ✅ | 健康检查、结构化日志、Prometheus 指标、优雅关闭 |
| 第三方 API 网关 | ⏳ | Redis 缓存 / 限流 / 重试 / 熔断、多 Key 轮询、调用日志 |
| 数据计算引擎 | ⏳ | Asynq 任务队列、进度 SSE 推送 |
| 实时通信 | ⏳ | WebSocket 推送、事件总线 |

⏳ 为二期计划，代码中已预留 `internal/config` 的 Redis 配置与 `model.APILog` 表。

---

## 技术栈

| 分类 | 选型 | 说明 |
|---|---|---|
| 语言 | Go 1.26.0 | `go.mod` 中 `go 1.26.0` |
| Web 框架 | [Gin](https://github.com/gin-gonic/gin) v1.12 | 高性能 HTTP 路由 |
| ORM | [GORM](https://gorm.io) v1.31 | 自动迁移、通用 CRUD |
| 配置 | [Viper](https://github.com/spf13/viper) v1.21 | YAML + 环境变量，显式 BindEnv |
| 日志 | [Zap](https://github.com/uber-go/zap) v1.28 | 结构化日志，JSON/Console 双格式 |
| 指标 | [Prometheus](https://prometheus.io) client v1.24 | 请求量 / 耗时 / 鉴权失败计数 |
| 认证 | JWT (HS256) + API Key | `golang-jwt/jwt/v5` + `x/crypto/bcrypt` |
| 数据库 | SQLite（纯 Go）/ PostgreSQL | `glebarez/sqlite`（无 CGO）/ `gorm postgres` |
| 容器 | Docker 多阶段构建 | Alpine 3.20 运行期，镜像 < 30MB |
| CI/CD | GitHub Actions | 推 TCR → SSH 部署 → Docker Compose |

---

## 快速开始

### 前置要求

- Go 1.26.0+（`go version` 确认）
- Make（可选，也可以用原生命令）

### 三步启动

```bash
# 1. 启动服务（默认 SQLite，零外部依赖）
make run
# 或
go run ./cmd/server -c config/config.yaml

# 2. 验证健康检查
curl http://localhost:8080/health
# {"code":0,"message":"success","data":{"status":"up","database":"up","uptime":"5s","version":"dev (commit=none, built=unknown)"}}

# 3. 登录获取令牌（首次启动自动创建管理员）
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"admin123456"}'
```

登录成功后返回的 `access_token` 后续放在 `Authorization: Bearer <token>` 头中使用：

```bash
curl http://localhost:8080/api/v1/auth/me \
  -H 'Authorization: Bearer eyJhbGciOi...'
```

> **生产环境**：必须用环境变量 `JWT_SECRET` 覆盖默认密钥（长度 ≥ 32，否则拒绝启动），
> 并修改管理员默认密码。详见 [.env.example](.env.example)。

---

## Docker Compose 开发环境

一键启动 Go 应用 + PostgreSQL + Redis 全套开发环境：

```bash
make up      # 构建镜像并启动全部服务
make logs    # 查看应用日志（实时跟踪）
make ps      # 查看容器状态
make down    # 停止（保留数据卷）
# 彻底清除数据
docker compose down -v
```

容器内自动切换为 PostgreSQL（通过 `DB_DRIVER` / `DB_DSN` 环境变量覆盖），
Redis 同步启用（`REDIS_ENABLED=true`）。

### 服务端口

| 服务 | 宿主机端口 | 容器端口 | 说明 |
|---|---|---|---|
| Go 应用 | 8080 | 8080 | 可通过 `APP_PORT` 修改 |
| PostgreSQL | 5432 | 5432 | 可通过 `POSTGRES_PORT` 修改 |
| Redis | 6379 | 6379 | 可通过 `REDIS_PORT` 修改 |

### 数据库连接信息（Docker Compose 环境）

```
host:     127.0.0.1
port:     5432
user:     postgres
password: postgres
dbname:   gobackend
```

连接字符串：

```
host=127.0.0.1 user=postgres password=postgres dbname=gobackend port=5432 sslmode=disable TimeZone=Asia/Shanghai
```

---

## 常用命令

| 命令 | 说明 |
|---|---|
| `make help` | 显示全部可用命令 |
| `make run` | 本地启动服务（读取 `config/config.yaml`） |
| `make dev` | 开发模式启动（debug 日志 + debug 模式） |
| `make build` | 编译当前平台可执行文件到 `bin/` |
| `make build-linux` | 交叉编译 Linux amd64 版本（部署用） |
| `make test` | 运行全部单元测试（带覆盖率） |
| `make test-short` | 运行测试（跳过耗时较长的用例） |
| `make cover` | 生成并打开 HTML 覆盖率报告 |
| `make lint` | 格式化 + 静态检查（`gofmt` + `go vet`） |
| `make check` | 提交前完整检查（lint + test） |
| `make docs` | 打印 API 文档地址 |
| `make docker-build` | 构建 Docker 镜像 |
| `make up` / `make down` | Docker Compose 开发环境启停 |
| `make logs` | 查看容器日志 |
| `make clean` | 清理构建产物与运行期数据 |
| `make tidy` | 整理 Go 依赖 |

---

## 目录结构

```
go-backend/
├── cmd/
│   └── server/
│       └── main.go                 # 程序入口：配置加载 → 日志初始化 → 依赖装配 → HTTP 启动 → 优雅关闭
├── internal/
│   ├── apperr/
│   │   ├── apperr.go               # 统一错误码体系（HTTP状态码 × 1000 + 序号）
│   │   └── apperr_test.go
│   ├── auth/
│   │   ├── crypto.go               # 密码哈希（bcrypt）、API Key 哈希（SHA-256）
│   │   ├── crypto_test.go
│   │   ├── jwt.go                  # JWT 签发与校验（HS256）
│   │   └── jwt_test.go
│   ├── bootstrap/
│   │   └── bootstrap.go            # 依赖装配（手写 DI）+ 首次启动种子数据
│   ├── config/
│   │   └── config.go               # Viper 配置加载：默认值 → YAML → 环境变量 → 校验
│   ├── database/
│   │   └── database.go             # GORM 建连、自动迁移、慢查询日志桥接
│   ├── docscheck/
│   │   └── drift_test.go           # API 文档与代码一致性检查
│   ├── handler/
│   │   ├── auth.go                 # 认证接口：登录、刷新令牌、获取当前用户、改密
│   │   ├── system.go               # 系统接口：健康检查、存活探针、运行时信息
│   │   └── user.go                 # 用户管理 + API Key 管理接口
│   ├── logger/
│   │   └── logger.go               # Zap 全局日志：Debugf/Infof/Warnf/Errorf/Fatalf
│   ├── metrics/
│   │   └── metrics.go              # Prometheus 指标：请求量/耗时/鉴权失败
│   ├── middleware/
│   │   ├── middleware.go            # RequestID / Logger / CORS / Recover / Timeout
│   │   ├── auth.go                 # JWT 认证 / API Key 认证 / RBAC / 限流
│   │   └── ratelimit.go            # 令牌桶限流器（按 IP，进程内）
│   ├── model/
│   │   ├── model.go                # 持久化实体：User / APIKey / APILog
│   │   └── dto/
│   │       └── dto.go              # 请求/响应 DTO：LoginRequest / PageQuery 等
│   ├── repository/
│   │   ├── repository.go           # 仓储接口定义
│   │   ├── user.go                 # 用户数据访问（interface + GORM 实现）
│   │   └── apikey.go               # API Key 数据访问
│   ├── response/
│   │   └── response.go             # 统一 JSON 响应：OK / Page / Fail / FailWithCode
│   ├── router/
│   │   └── router.go               # 路由注册与全局中间件装配
│   ├── service/
│   │   ├── user.go                 # 用户业务逻辑
│   │   └── apikey.go               # API Key 业务逻辑
│   └── version/
│       └── version.go              # 构建期注入版本号（ldflags）
├── docs/
│   ├── docs.go                     # OpenAPI 3.1 spec 构造（Go struct 手写）
│   ├── paths.go                    # API 路径定义
│   ├── schemas.go                  # 数据模型定义
│   ├── helpers.go                  # 辅助函数
│   ├── ui.go                       # Scalar UI 页面
│   └── smoke_test.go
├── config/
│   └── config.yaml                 # 非敏感配置文件
├── deploy/                         # 部署相关文件
├── .github/
│   └── workflows/
│       ├── ci.yml                  # CI：gofmt → go vet → golangci-lint → build → test
│       └── deploy.yml              # 部署：复用 CI → 构建镜像 → 推 TCR → SSH 部署
├── .env.example                    # 环境变量示例（不提交 .env）
├── .gitignore
├── .golangci.yml                   # golangci-lint 配置
├── Dockerfile                      # 多阶段构建：golang:1.26-alpine → alpine:3.20
├── Makefile                        # 常用命令集合
├── docker-compose.yaml             # 开发环境编排（Go + Postgres + Redis）
├── compose.prod.yaml               # 生产部署编排（仅应用，镜像由 CI 提供）
├── go.mod
├── go.sum
└── README.md
```

---

## 请求处理流程

```
客户端请求
  │
  ▼
┌──────────────────────────────────────────────────────────┐
│ 全局中间件（顺序敏感）                                      │
│  RequestID → Recover → Logger → CORS → Metrics           │
└──────────────────────────────────────────────────────────┘
  │
  ▼
┌──────────────────────────────────────────────────────────┐
│ 路由匹配                                                  │
│  公开区: /health, /ready, /metrics, /docs                │
│  公开区: POST /auth/login (限流), POST /auth/refresh      │
│  JWT 区: middleware.JWT() → handler                       │
│  Admin 区: middleware.JWT() + RequireAdmin() → handler    │
│  API Key 区: middleware.APIKey() → handler                │
└──────────────────────────────────────────────────────────┘
  │
  ▼
┌──────────────────────────────────────────────────────────┐
│ Handler 层 (handler/)                                     │
│  1. 绑定并校验请求参数（binding tag）                        │
│  2. 调用 Service                                          │
│  3. 用 response 包封装统一格式返回                           │
└──────────────────────────────────────────────────────────┘
  │
  ▼
┌──────────────────────────────────────────────────────────┐
│ Service 层 (service/)                                     │
│  业务逻辑、权限判断、数据转换                                 │
└──────────────────────────────────────────────────────────┘
  │
  ▼
┌──────────────────────────────────────────────────────────┐
│ Repository 层 (repository/)                               │
│  interface + GORM 实现，数据持久化                           │
└──────────────────────────────────────────────────────────┘
  │
  ▼
数据库 (SQLite / PostgreSQL)
```

**分层调用方向**：`handler → service → repository`，不允许反向依赖。
跨层传对象时，`handler → service` 传 DTO，`service → repository` 传实体。

---

## 统一响应格式

所有接口一律返回以下 JSON 结构：

### 成功响应

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "id": 1,
    "username": "admin"
  }
}
```

### 分页响应

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "list": [
      {"id": 1, "username": "admin", "role": "admin"}
    ],
    "total": 100,
    "page": 1,
    "page_size": 20
  }
}
```

### 失败响应

```json
{
  "code": 401002,
  "message": "用户名或密码错误",
  "data": null
}
```

**规则**：

- `code == 0` 表示成功，非 0 一律失败
- `data` 为 `null` 时不出现在 JSON 中（实际返回 `null`）
- **5xx 错误对外统一返回「服务器内部错误」**，真实原因只写服务端日志
- 前端按 `code` 字段分支处理

---

## 错误码体系

规则：**`HTTP 状态码 × 1000 + 序号`**

从错误码可直接反查 HTTP 状态（`code / 1000` 即 HTTP 状态码），客户端无需额外映射表。

| 错误码 | HTTP | 含义 |
|---|---|---|
| `0` | 200 | 成功 |
| `400001` | 400 | 参数校验失败 |
| `401001` | 401 | 令牌无效或已过期 |
| `401002` | 401 | 用户名或密码错误 |
| `401003` | 401 | API Key 已禁用 |
| `403001` | 403 | 缺少访问权限 |
| `404000` | 404 | 资源不存在（接口不存在） |
| `405000` | 405 | 请求方法不允许 |
| `409001` | 409 | 用户名已存在 |
| `409002` | 409 | API Key 别名已存在 |
| `429000` | 429 | 触发限流 |
| `500000` | 500 | 服务器内部错误（兜底） |
| `500001` | 500 | 数据库操作失败 |
| `500002` | 500 | 第三方 API 调用失败 |
| `500003` | 500 | 加解密失败 |
| `500004` | 500 | 文件上传失败 |
| `503000` | 503 | 服务暂时不可用 |

完整定义见 [internal/apperr/apperr.go](internal/apperr/apperr.go)。

---

## API 接口一览

启动后访问交互式 API 文档：

- **Scalar UI**：`http://localhost:8080/docs`
- **OpenAPI 3.1 Spec**：`http://localhost:8080/openapi.json`

> `APP_MODE=release` 时以上路由不注册，避免暴露接口细节。

### 公开接口（无需鉴权）

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/health` | 健康检查（含数据库连通性，失败返回 503） |
| GET | `/ready` | 存活探针（不查依赖，供 K8s livenessProbe） |
| GET | `/metrics` | Prometheus 抓取端点 |
| POST | `/api/v1/auth/login` | 登录（限流：1 req/s，桶容量 5） |
| POST | `/api/v1/auth/refresh` | 用刷新令牌换新令牌对（限流：1 req/s，桶容量 10） |

#### 登录示例

```bash
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{
    "username": "admin",
    "password": "admin123456"
  }'
```

响应：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "access_token": "eyJhbGciOi...",
    "refresh_token": "eyJhbGciOi...",
    "expires_in": 7200,
    "token_type": "Bearer",
    "user": {
      "id": 1,
      "username": "admin",
      "nickname": "超级管理员",
      "role": "admin",
      "email": "",
      "status": 1
    }
  }
}
```

#### 刷新令牌示例

```bash
curl -X POST http://localhost:8080/api/v1/auth/refresh \
  -H 'Content-Type: application/json' \
  -d '{"refresh_token": "eyJhbGciOi..."}'
```

---

### JWT 鉴权接口

所有接口需在请求头携带 `Authorization: Bearer <access_token>`。

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/v1/auth/me` | 获取当前登录用户信息 |
| PUT | `/api/v1/auth/password` | 修改密码 |
| GET | `/api/v1/system/info` | 运行时信息（版本 / goroutine / 内存） |
| GET | `/api/v1/apikeys` | API Key 列表 |
| POST | `/api/v1/apikeys` | 创建 API Key（明文只返回一次） |
| PATCH | `/api/v1/apikeys/:id` | 启用 / 禁用 API Key |
| DELETE | `/api/v1/apikeys/:id` | 删除 API Key |

#### 获取当前用户

```bash
curl http://localhost:8080/api/v1/auth/me \
  -H 'Authorization: Bearer <access_token>'
```

#### 修改密码

```bash
curl -X PUT http://localhost:8080/api/v1/auth/password \
  -H 'Authorization: Bearer <access_token>' \
  -H 'Content-Type: application/json' \
  -d '{
    "old_password": "admin123456",
    "new_password": "newpass123456"
  }'
```

#### 查看运行时信息

```bash
curl http://localhost:8080/api/v1/system/info \
  -H 'Authorization: Bearer <access_token>'
```

响应：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "version": "sha-abc1234 (commit=abc1234..., built=2026-09-29T...)",
    "go_version": "go1.26.0",
    "goroutines": 15,
    "uptime": "2h30m15s",
    "mem": {
      "alloc_mb": 12.34,
      "total_alloc_mb": 56.78,
      "sys_mb": 30.12,
      "num_gc": 8
    }
  }
}
```

#### 创建 API Key

```bash
curl -X POST http://localhost:8080/api/v1/apikeys \
  -H 'Authorization: Bearer <access_token>' \
  -H 'Content-Type: application/json' \
  -d '{
    "name": "my-monitoring-tool",
    "role": "viewer",
    "days": 90
  }'
```

> **注意**：`key` 字段只在创建时返回一次，之后无法再次获取明文。请妥善保存。

---

### 管理员接口（需 admin 角色）

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/v1/users` | 用户列表（分页 + 关键字 + 角色过滤） |
| POST | `/api/v1/users` | 创建用户 |
| GET | `/api/v1/users/:id` | 用户详情 |
| PUT | `/api/v1/users/:id` | 更新用户（只改传入的字段） |
| DELETE | `/api/v1/users/:id` | 删除用户（不能删自己） |

#### 创建用户

```bash
curl -X POST http://localhost:8080/api/v1/users \
  -H 'Authorization: Bearer <access_token>' \
  -H 'Content-Type: application/json' \
  -d '{
    "username": "alice",
    "password": "pass123456",
    "nickname": "爱丽丝",
    "role": "editor",
    "email": "alice@example.com"
  }'
```

#### 用户列表（分页 + 过滤）

```bash
curl "http://localhost:8080/api/v1/users?page=1&page_size=20&keyword=adm&role=admin" \
  -H 'Authorization: Bearer <access_token>'
```

#### 更新用户

```bash
curl -X PUT http://localhost:8080/api/v1/users/2 \
  -H 'Authorization: Bearer <access_token>' \
  -H 'Content-Type: application/json' \
  -d '{
    "nickname": "新昵称",
    "role": "viewer"
  }'
```

---

### API Key 鉴权接口

供外部工具调用，通过 `X-API-Key` 请求头传递密钥。

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/v1/tool/ping` | 鉴权链路自检 |

```bash
curl http://localhost:8080/api/v1/tool/ping \
  -H 'X-API-Key: gk_ab12cd34...'
```

响应：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "pong": true,
    "role": "viewer"
  }
}
```

---

## 认证机制

本项目支持两种认证方式，共用同一套 RBAC 权限判定：

### JWT 认证（人工调用）

适用于浏览器 / 前端应用。流程：

1. 调用 `POST /api/v1/auth/login` 获取 `access_token` + `refresh_token`
2. 后续请求携带 `Authorization: Bearer <access_token>`
3. `access_token` 过期后（默认 2 小时），用 `refresh_token` 调用 `POST /api/v1/auth/refresh` 获取新令牌对

**令牌配置**：

| 参数 | 默认值 | 环境变量 |
|---|---|---|
| 签名算法 | HS256 | - |
| 密钥 | `dev-only-secret-change-me-...` | `JWT_SECRET`（**生产必须替换，长度 ≥ 32**） |
| 访问令牌有效期 | 2h | `JWT_ACCESS_TTL` |
| 刷新令牌有效期 | 168h（7 天） | `JWT_REFRESH_TTL` |

生成安全的 JWT 密钥：

```bash
openssl rand -hex 32
# 或
head -c 48 /dev/urandom | base64 | tr -d '\n'
```

### API Key 认证（机器调用）

适用于外部工具、脚本、监控系统。流程：

1. 登录后调用 `POST /api/v1/apikeys` 创建密钥
2. 创建时返回明文 `key`（形如 `gk_xxxx`），**只返回一次**
3. 后续请求携带 `X-API-Key: gk_xxxx`

也支持 `Authorization: ApiKey <token>` 写法（兼容格式）。

**API Key 特性**：

- 支持设置过期时间（`days` 参数，0 表示永不过期）
- 支持启用/禁用（`PATCH /api/v1/apikeys/:id`）
- 密钥在数据库中只存前缀 + 哈希，与密码存储策略一致
- 归属校验：非 admin 用户只能操作自己创建的密钥，越权返回 **404**（而非 403，避免泄露「该 ID 存在」）

### 角色权限（RBAC）

| 角色 | 说明 |
|---|---|
| `admin` | 超级管理员，恒通过所有 `RequireRole` 校验 |
| `editor` | 编辑者，可读写业务数据 |
| `viewer` | 访客，只读 |

- 鉴权中间件把身份写入 context，handler 通过 `middleware.GetUserID(c)` / `GetRole(c)` 读取
- 人工调用（JWT）与机器调用（API Key）共用同一套 RBAC 判定

---

## 数据库

### SQLite（默认，零依赖）

适合单机开发与小型部署，使用纯 Go 实现（`glebarez/sqlite`），无需 CGO 与 C 工具链。

```yaml
# config/config.yaml
database:
  driver: sqlite
  dsn: ./data/app.db
  max_open_conns: 20
  max_idle_conns: 5
  conn_max_lifetime: 30m
  slow_threshold: 200ms
```

SQLite 参数已固定：WAL 模式 + `busy_timeout=5000` + 外键开启 + `synchronous=NORMAL`。

**限制**：并发写入会串行化（单写者），读多写少够用，写密集场景请切 PostgreSQL。

### PostgreSQL（生产推荐）

```yaml
database:
  driver: postgres
  dsn: "host=127.0.0.1 user=postgres password=xxx dbname=gobackend port=5432 sslmode=disable TimeZone=Asia/Shanghai"
```

或通过环境变量：

```bash
DB_DRIVER=postgres
DB_DSN="host=127.0.0.1 user=postgres password=xxx dbname=gobackend port=5432 sslmode=disable"
```

### 数据库表

应用启动时自动执行 `AutoMigrate`，创建以下表：

| 表名 | 说明 |
|---|---|
| `users` | 用户表（ID、用户名、密码哈希、昵称、角色、邮箱、状态） |
| `api_keys` | API Key 表（ID、别名、密钥前缀、密钥哈希、归属用户、角色、启用状态） |
| `api_logs` | 第三方 API 调用日志（二期使用） |

### 默认管理员账号

首次启动时自动创建（`app.bootstrap.enabled=true`）：

| 用户名 | 密码 | 角色 |
|---|---|---|
| `admin` | `admin123456` | `admin` |

> **生产环境**：必须修改默认密码，或关闭 `app.bootstrap.enabled`。

---

## 配置管理

### 加载顺序

**内置默认值 → `config/config.yaml` → 环境变量**（后者覆盖前者）

配置文件默认路径 `config/config.yaml`，可通过 `-c` 参数或 `CONFIG_PATH` 环境变量指定。

### 环境变量

敏感信息一律走环境变量，显式绑定的变量名如下：

| 环境变量 | 配置键 | 说明 |
|---|---|---|
| `APP_NAME` | `app.name` | 应用名称 |
| `APP_MODE` | `app.mode` | `debug` / `release` / `test` |
| `APP_PORT` | `app.port` | 监听端口（默认 8080） |
| `APP_UPLOAD_DIR` | `app.upload_dir` | 文件上传目录 |
| `APP_CORS_ORIGINS` | `app.cors_origins` | CORS 白名单（逗号分隔） |
| `JWT_SECRET` | `app.jwt.secret` | JWT 签名密钥（**长度 ≥ 32**） |
| `JWT_ACCESS_TTL` | `app.jwt.access_ttl` | 访问令牌有效期 |
| `JWT_REFRESH_TTL` | `app.jwt.refresh_ttl` | 刷新令牌有效期 |
| `LOG_LEVEL` | `log.level` | `debug` / `info` / `warn` / `error` |
| `LOG_FORMAT` | `log.format` | `console` / `json` |
| `LOG_FILE` | `log.file` | 日志文件路径（空则只输出到 stdout） |
| `DB_DRIVER` | `database.driver` | `sqlite` / `postgres` |
| `DB_DSN` | `database.dsn` | 数据库连接串 |
| `REDIS_ENABLED` | `redis.enabled` | 是否启用 Redis |
| `REDIS_ADDR` | `redis.addr` | Redis 地址 |
| `REDIS_PASSWORD` | `redis.password` | Redis 密码 |

### 配置校验

`Validate()` 不通过直接返回 error，启动即失败：

- `app.port` ∈ [1, 65535]
- `app.mode` ∈ {debug, release, test}
- `app.jwt.secret` 长度 ≥ 32
- `refresh_ttl` ≥ `access_ttl`
- `database.driver` ∈ {sqlite, postgres}
- `log.level` ∈ {debug, info, warn, error}

### 完整配置参考

详见 [config/config.yaml](config/config.yaml) 与 [.env.example](.env.example)。

---

## 部署

### 部署链路

```
push main → CI (ci.yml) → Deploy (deploy.yml)
  │                          │
  │                          ├─ 构建 Docker 镜像
  │                          ├─ 推送到腾讯云 TCR
  │                          ├─ SSH 到服务器
  │                          └─ docker compose up -d
  │
  ├─ gofmt 格式检查
  ├─ go vet 静态分析
  ├─ golangci-lint
  ├─ go build 编译
  └─ go test 测试
```

### Docker 镜像构建

多阶段构建，最终镜像基于 Alpine 3.20，体积 < 30MB：

```bash
# 本地构建
docker build -t go-backend:latest .

# 或通过 Make
make docker-build
```

构建参数（通过 `--build-arg` 注入）：

| 参数 | 说明 |
|---|---|
| `VERSION` | 版本号，注入到 `internal/version.Version` |
| `COMMIT` | Git commit hash |
| `BUILD_TIME` | 构建时间 |

### 交叉编译（不使用 Docker）

```bash
# 编译 Linux amd64 版本
make build-linux
# 产出: bin/server-linux-amd64
```

SQLite 驱动使用纯 Go 实现，`CGO_ENABLED=0` 全局生效，无需 C 工具链。

### 生产部署（compose.prod.yaml）

服务器端使用 `compose.prod.yaml`，镜像地址通过 `IMAGE` 环境变量注入：

```bash
# 服务器上
IMAGE=ccr.ccs.tencentyun.com/my-ns/go-backend:sha-abc1234 \
docker compose -f compose.prod.yaml up -d
```

生产环境要点：

- `APP_MODE=release`（关闭调试输出、不注册 `/docs` 与 `/openapi.json`）
- `JWT_SECRET` 必须通过 `.env` 设置（部署脚本会拦截默认密钥）
- `LOG_FORMAT=json`（便于日志采集）
- 日志驱动限大小：单文件 10MB，最多 3 个（共 30MB）
- 健康检查：每 30s 请求 `/health`，3 次失败判定为不健康

### GitHub Actions Secrets

需要在仓库 Settings → Secrets 中配置：

| Secret | 说明 | 示例 |
|---|---|---|
| `TCR_REGISTRY` | 腾讯云 TCR 仓库地址 | `ccr.ccs.tencentyun.com/my-ns` |
| `TCR_IMAGE` | 镜像名 | `go-backend` |
| `TCR_USERNAME` | TCR 用户名 | |
| `TCR_PASSWORD` | TCR 密码 | |
| `DEPLOY_HOST` | 服务器地址 | `1.2.3.4` |
| `DEPLOY_USER` | SSH 用户名 | `ubuntu` |
| `DEPLOY_SSH_KEY` | SSH 私钥全文 | |
| `DEPLOY_PATH` | 服务器部署目录 | `/opt/go-backend` |

### 回滚

```bash
# SSH 到服务器
ssh deploy-user@server-ip
cd /opt/go-backend

# 查看可用镜像版本
docker images --format '{{.Repository}}:{{.Tag}}' | head

# 切换到上一版本
IMAGE=<上一版sha标签> docker compose -f compose.prod.yaml up -d

# 查看日志排查问题
docker compose -f compose.prod.yaml logs --tail=100 app
```

---

## 中间件顺序

```
RequestID → Recover → Logger → CORS → Metrics → 路由匹配 → 组内(JWT / RateLimiter / RequireAdmin / APIKey)
```

| 中间件 | 说明 |
|---|---|
| `RequestID` | 最先执行，注入 `X-Request-ID`（上游传入时沿用），后续所有日志与响应都带这个 ID |
| `Recover` | 紧随其后，把 panic 转成标准 500 响应而非直接掐断连接 |
| `Logger` | 记录请求方法、路径、状态码、耗时、IP，依赖 RequestID 做全链路关联 |
| `CORS` | 处理跨域预检与放行，支持通配符 `*` 或白名单匹配 |
| `Metrics` | 记录 Prometheus 指标（path 使用路由模板避免基数爆炸） |
| `JWT` | 校验 `Authorization: Bearer <token>`，通过后写入 user_id / role 到 context |
| `APIKey` | 校验 `X-API-Key` 头，通过后写入 user_id / role（与 JWT 共享 RBAC） |
| `RateLimiter` | 令牌桶限流（按 IP），登录接口 1 req/s 桶容量 5 |
| `RequireAdmin` | 要求 admin 角色，admin 恒通过 |

---

## 常见问题排查

### 启动失败：`app.jwt.secret 长度必须 >= 32`

JWT 密钥过短，使用环境变量覆盖：

```bash
JWT_SECRET="$(openssl rand -hex 32)" make run
```

### 启动失败：`database.driver 只能是 sqlite/postgres`

检查 `config/config.yaml` 或 `DB_DRIVER` 环境变量，确保值为 `sqlite` 或 `postgres`。

### SQLite 并发写入慢

SQLite 单写者限制，写密集场景请切换到 PostgreSQL：

```bash
DB_DRIVER=postgres DB_DSN="host=127.0.0.1 user=postgres password=xxx dbname=gobackend port=5432 sslmode=disable" make run
```

### Docker Compose 启动后数据库连不上

检查 PostgreSQL 容器是否健康：

```bash
docker compose ps
# 确认 postgres 状态为 healthy

# 手动测试连接
docker exec -it go-backend-postgres pg_isready -U postgres -d gobackend
```

### 登录后请求返回 401

- 检查 `Authorization` 头格式：`Bearer <token>`（注意空格）
- 检查令牌是否过期（默认 2 小时），过期后用 `refresh_token` 刷新
- 检查 `JWT_SECRET` 是否与签发时一致

### API Key 返回 401

- 检查请求头：`X-API-Key: gk_xxxx`
- 检查密钥是否已禁用：`GET /api/v1/apikeys` 查看 `enabled` 字段
- 检查密钥是否已过期

### 部署时健康检查超时

```bash
# SSH 到服务器查看日志
docker compose -f compose.prod.yaml logs --tail=100 app

# 手动测试健康检查
curl http://127.0.0.1:8080/health
```

常见原因：数据库连接失败、`JWT_SECRET` 未设置、端口被占用。

### CI 失败：gofmt 格式检查

```bash
# 本地格式化
make fmt
# 或
gofmt -w -s .
```

### 交叉编译失败

本项目全局 `CGO_ENABLED=0`（纯 Go SQLite），无需 C 工具链：

```bash
make build-linux
```

如果仍失败，检查 Go 版本是否 ≥ 1.26.0（`go version`）。

---

## 本地开发快速启动

```bash
# 1. 克隆仓库
git clone git@github.com:Ezily-y/go-backend.git && cd go-backend

# 2. 整理依赖
go mod tidy

# 3. 启动服务（默认 SQLite）
make run

# 4. 另一个终端：登录
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"admin123456"}'

# 5. 保存 access_token，后续请求使用
TOKEN="<从上一步获取的 access_token>"
curl http://localhost:8080/api/v1/auth/me -H "Authorization: Bearer $TOKEN"
```

---

## License

Internal project.