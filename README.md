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
- [从改代码到上线（完整流程）](#从改代码到上线完整流程)
- [踩坑记录](#踩坑记录)
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

⏳ 为二期计划，代码中已预留 `internal/core/config` 的 Redis 配置与 `model.APILog` 表。

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
- Make（可选；本机没装的话直接用下面的原生命令）

### 克隆仓库

```bash
git clone git@github.com:Ezily-y/go-backend.git && cd go-backend
go mod tidy
```

> 没配 SSH key 就改用 HTTPS：`git clone https://github.com/Ezily-y/go-backend.git`

### 三步启动

```bash
# 1. 启动服务（默认 SQLite，零外部依赖）
make run
# 或（本机没装 make 时）
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
│   ├── deploy.sh                   # 服务器上执行的部署脚本（CI scp 下发）
│   └── server-setup.sh             # 新服务器一次性初始化（装 Docker、建目录、生成 .env）
├── memory/                         # 踩坑记录（随仓库走，新经验写这里）
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

完整定义见 [internal/core/apperr/apperr.go](internal/core/apperr/apperr.go)。

---

## API 接口一览

启动后访问交互式 API 文档：

- **Scalar UI**：`http://localhost:8080/docs`
- **OpenAPI 3.1 Spec**：`http://localhost:8080/openapi.json`

> 由 `app.docs_enabled` 单独控制，与 `app.mode` 解耦（默认开）。
> 生产 `compose.prod.yaml` 传 `DOCS_ENABLED=false` 默认关闭，避免暴露接口结构；
> 需要临时对外展示时在服务器 `.env` 加 `DOCS_ENABLED=true` 即可，
> 不必把 `APP_MODE` 退回 `debug`（那会连带开 gin 调试输出、改日志格式）。

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

- `APP_MODE=release`（关闭 gin 调试输出）
- `DOCS_ENABLED=false`（`compose.prod.yaml` 默认值，不注册 `/docs` 与 `/openapi.json`）
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

## 从改代码到上线（完整流程）

> 写给第一次接触 CI/CD 的人。整条链路只需要你记住一句话：
>
> **改代码 → 本地跑通 → `git add` → `git commit` → `git push` → 剩下的全自动。**
>
> 下面把"剩下的全自动"拆开讲清楚，这样挂了你知道去哪看。

### 0. 全景图

```
┌─────────────┐  push   ┌──────────────────────────────┐  拉镜像  ┌────────────────┐
│  你的电脑     │ ─────▶ │  GitHub Actions               │ ─────▶ │  腾讯云服务器     │
│             │         │                              │        │                │
│ · 改代码     │         │ ① CI（质量门禁）               │        │ ④ docker compose│
│ · 本地自测   │         │   gofmt → vet → lint          │        │    pull 新镜像  │
│ · git commit│         │   → build → test             │        │    重启容器     │
│ · git push  │         │   任一步失败 ⇒ 整条流水线停止   │        │    等 /health  │
│             │         │                              │        │                │
└─────────────┘         │ ② 构建 Docker 镜像            │        │    数据持久化在  │
                        │ ③ 推送到腾讯云 TCR 镜像仓库     │        │    ./data 目录  │
                        └──────────────────────────────┘        └────────────────┘
```

**关键点：你只做 `git push` 这一个动作，剩下的全自动。**

- 测试没过 → 停在 ①，**生产环境一个字节都不会被碰到**
- 测试过了 → 自动构建镜像 → 自动推到 TCR → 自动 SSH 到服务器滚动重启
- 每个镜像都带 `sha-<7位哈希>` 标签，随时可以精确回滚到任意一次提交

三份文件负责整条链：

| 文件 | 干什么 |
|---|---|
| [.github/workflows/ci.yml](.github/workflows/ci.yml) | 质量门禁：格式 / 静态检查 / Lint / 编译 / 测试 |
| [.github/workflows/deploy.yml](.github/workflows/deploy.yml) | 先跑上面那份 CI，过了才构建镜像并部署 |
| [deploy/deploy.sh](deploy/deploy.sh) | 真正在服务器上执行的部署脚本（由 workflow scp 下发） |

---

### 1. 第一次准备（只做一次）

#### 1.1 腾讯云侧

**a) 开通容器镜像服务 TCR，建一个命名空间和仓库**

控制台 → 容器镜像服务 → 镜像仓库 → 新建命名空间（如 `my-ns`）→ 新建私有仓库 `go-backend`。
记下三样东西：**公网访问地址**、**访问凭证用户名**、**访问凭证密码**。

> TCR 的用户名密码是**独立生成的一套**，不是腾讯云账号密码，也不是 SecretId/SecretKey。

**b) 一次性初始化服务器**

```bash
# 本机执行：把初始化脚本传上去
scp deploy/server-setup.sh ubuntu@<服务器IP>:/tmp/

# 服务器上执行（装 Docker、配镜像加速、建部署目录、生成 .env 模板）
ssh ubuntu@<服务器IP> 'sudo bash /tmp/server-setup.sh'
```

脚本会装好 Docker + compose 插件、配置腾讯云内网镜像加速、创建 `/opt/go-backend`，
并生成一份 `.env` 模板。**它不覆盖已有 `.env`，可以重复执行。**

**c) 填服务器密钥（必须做，否则部署会被拒绝）**

```bash
ssh ubuntu@<服务器IP>
cd /opt/go-backend
nano .env                 # 把 JWT_SECRET 填上
# 生成随机值：openssl rand -hex 32
```

> `JWT_SECRET` 长度必须 ≥ 32，而且**不能**是 `.env.example` 里的
> `dev-only-secret-change-me-0123456789`（公开已知值，谁都能伪造你的登录态）。
> `deploy/deploy.sh` 会检测到样例值直接拒绝部署 —— 这是故意的。
>
> 这个密钥**只存在服务器的 `.env` 里**，不进 GitHub、不进代码、不进 git。

**d) 云安全组放行 8080 端口**

腾讯云控制台 → 防火墙/安全组 → 添加规则 → TCP 8080。
`server-setup.sh` 只管本机 ufw，**管不到云控制台的安全组**，这一步必须手动。

**e) 生成一把专用 SSH 密钥**

```bash
# 本机执行（如果还没有专用密钥）
ssh-keygen -t ed25519 -f ~/.ssh/go-backend-deploy -N "" -C "go-backend-deploy"

# 把公钥装到服务器，让 CI 能免密登录
cat ~/.ssh/go-backend-deploy.pub >> ~/.ssh/authorized_keys   # 在服务器上执行
```

> 用**专用**密钥，别拿日常那把 —— 私钥全文要贴进 GitHub Secrets，
> 泄露时只废这一把，不影响你平时登录。

#### 1.2 GitHub 侧

仓库 → **Settings → Secrets and variables → Actions → New repository secret**。
名字必须**一字不差**，`deploy.yml` 就是按这些名字取值的：

| Secret | 值 | 从哪拿 |
|---|---|---|
| `TCR_REGISTRY` | `ccr.ccs.tencentyun.com/<命名空间>` | TCR 控制台"公网访问地址"，**不带 `https://`、不带尾 `/`** |
| `TCR_IMAGE` | `go-backend` | TCR 仓库名 |
| `TCR_USERNAME` | TCR 访问凭证用户名 | TCR 控制台"访问凭证"页 |
| `TCR_PASSWORD` | TCR 访问凭证密码 | 同上 |
| `DEPLOY_HOST` | 服务器公网 IP | 腾讯云控制台 |
| `DEPLOY_USER` | 登录用户名 | 轻量服务器一般 `ubuntu`，CVM 看镜像 |
| `DEPLOY_SSH_KEY` | **私钥全文** | `cat ~/.ssh/go-backend-deploy`（注意是私钥，不是 `.pub`） |
| `DEPLOY_PATH` | `/opt/go-backend` | 与 `server-setup.sh` 一致 |

> **`JWT_SECRET` 不是 GitHub Secret。** 它在服务器的 `.env` 里。
> CI 只负责把镜像推上去再重启容器，从头到尾拿不到应用密钥。

**粘贴私钥时注意**：Windows Git Bash 里直接 `cat` 会因为 MSYS 路径转换出问题，
用 stdin 传更稳：

```bash
printf '%s' "$(cat ~/.ssh/go-backend-deploy)" | gh secret set DEPLOY_SSH_KEY
# 或在网页上手动粘贴，确保首尾的 -----BEGIN/END OPENSSH PRIVATE KEY----- 都在
```

#### 1.3 验证一切就绪

```bash
# 本机
docker info                          # Docker Desktop 得开着
git remote -v                        # 确认是 git@github.com:Ezily-y/go-backend.git
git status                           # 确认在 main 分支
gh secret list                       # 确认 8 个 Secret 都在
```

---

### 2. 日常开发循环（每次都走这个）

#### 步骤一：切个分支（可选但推荐）

```bash
git checkout -b feat/xxx     # 改功能
git checkout -b fix/yyy      # 修 bug
```

> 不想用分支、直接在 `main` 上改也行 —— 但 `push main` 就**会直接部署到生产**。
> 想先看效果再上线，就开分支提 PR，PR 合并进 `main` 时才触发部署。

#### 步骤二：改代码

新增接口 / 加配置项 / 写测试，按项目约定走（详见 [CLAUDE.md](CLAUDE.md)）。
几个必踩的约定：

- 新增配置项要**同时改四处**（结构体 / `setDefaults` / `bindEnvs` / `config.yaml`），少一处就是配置不生效。
- 新增数据库实体必须加进 `database.Migrate()` 的 `targets`，否则表不会建。
- 统一响应用 `response.OK` / `response.Fail`，不要自己 `c.JSON` 拼。

#### 步骤三：本地自测（**别跳过**，这一步能挡掉 90% 的 CI 失败）

```bash
gofmt -l .          # 必须无输出；有输出就 gofmt -w -s .
go vet ./...        # 静态分析
go test ./...       # 跑测试（不要加 -race，本机不支持）
go build ./...      # 确认能编译
```

然后起服务看一眼：

```bash
go run ./cmd/server
# 另开一个终端
curl http://localhost:8080/health     # 注意是 /health，不是 /healthz
```

> 本机（Windows）**没装 make 和 golangci-lint**，直接敲上面这些原始 go 命令。
> `golangci-lint` 由 CI 在 Linux 上跑，规则在 [.golangci.yml](.golangci.yml)（v2 schema）。

#### 步骤四：提交

```bash
git add -A
git status                      # 先看清楚要提交什么
git commit -m "feat(xxx): 一句话说清做了什么"
```

commit message 用 `feat:` / `fix:` / `docs:` / `refactor:` / `chore:` 开头 + 中文描述，
说清**为什么这么改**，别只写"改了代码"。

> `.env`、`data/`、`logs/` 已在 [.gitignore](.gitignore) 里，不会被带进仓库。
> 提交前 `git status` 扫一眼，**确认没有密钥类文件**。

#### 步骤五：推送

```bash
git push origin main            # 或 git push origin feat/xxx
```

**到这里你的活就干完了。** 剩下的全自动。

---

### 3. 推送之后发生什么

去 GitHub → 仓库 → **Actions** 标签页，能看到两条流水线在跑：

```
CI       ─── gofmt → go vet → golangci-lint → go build → go test
Deploy   ─── 质量门禁(复用上面那条 CI) → 构建镜像 → 推 TCR → SSH 部署
```

点进去能看每一步的实时日志。

**成功的标志**：

1. 两条流水线都是绿色 ✅
2. Deploy 的"部署"步骤打印出 `健康检查通过 (等待 Ns)`
3. 本机验证一下：

```bash
# 直接访问（如果 8080 对外开放）
curl http://<服务器IP>:8080/health

# 或者 SSH 上去查本机
ssh ubuntu@<服务器IP> 'curl -s http://127.0.0.1:8080/health'
```

`version` 字段会回显 `sha-xxxxxxx`，**和你这次提交的短哈希一致**就说明线上跑的确实是新版本。

**挂了怎么办**：

| 现象 | 去哪看 | 常见原因 |
|---|---|---|
| CI 红了 | Actions → CI → 红色那步的日志 | 格式没格式化、测试没过、lint 报错 |
| 构建红了 | Deploy → "构建镜像"步骤 | Dockerfile 写错、依赖拉不下来 |
| 推镜像红了 | 同上，`docker login` 那步 | TCR 用户名/密码 Secret 填错 |
| 部署红了 | Deploy → "部署"步骤 | SSH 连不上、`.env` 缺失、JWT_SECRET 是样例值 |
| 部署成功但服务没起来 | SSH 上去 `docker compose -f compose.prod.yaml logs --tail=100 app` | 看下面[踩坑记录](#踩坑记录) |

> **注意**：CI 失败时 Deploy 根本不会往下走（`needs: quality-gate`），
> 所以 CI 红了 = 生产没被碰过，放心修完再推。

---

### 4. 回滚

线上出问题不用回滚代码，回滚**镜像**即可（几十秒）：

```bash
ssh ubuntu@<服务器IP>
cd /opt/go-backend

# 看历史镜像
docker images --format '{{.Repository}}:{{.Tag}}' | head

# IMAGE 由 deploy.sh 写进 .env；改成上一个 sha-* 标签
nano .env                      # IMAGE=ccr.ccs.tencentyun.com/my-ns/go-backend:sha-上一版
docker compose -f compose.prod.yaml pull app
docker compose -f compose.prod.yaml up -d

# 看日志确认
docker compose -f compose.prod.yaml logs --tail=100 app
```

每个镜像都带 `sha-<7位哈希>` 标签（不是只有 `latest`），就是为了这一步能精确定位。
嫌 SSH 麻烦，也可以去 GitHub → Actions → 找到上次成功的那次 → **Re-run jobs** 重新部署。

---

### 5. 常用运维命令速查

```bash
# —— 服务器上 ——
cd /opt/go-backend
docker compose -f compose.prod.yaml ps          # 容器状态（看 healthy）
docker compose -f compose.prod.yaml logs -f app # 跟踪日志
docker compose -f compose.prod.yaml restart app # 只重启不换镜像
docker compose -f compose.prod.yaml down        # 停掉
docker exec -it go-backend sh                   # 进容器内部

# —— 本机 ——
gh run list                 # 看最近的 CI/Deploy 运行结果
gh run view <run-id> --log   # 看某次运行的完整日志
gh secret list               # 看已配置的 Secrets
git log --oneline -10        # 看最近提交
```

---

## 踩坑记录

项目自带的踩坑笔记在 [`memory/`](memory/) 目录，**随仓库一起走**，换机器、换人都不会丢。
完整的排查手册见 [memory/README.md](memory/README.md)。

最常踩的三条（详细根因与修法看对应文件）：

| 坑 | 症状 | 一句话修法 |
|---|---|---|
| MSYS 路径转换 | `DEPLOY_PATH` 存成了 `D:/MySoftwares/Git/opt/...`，服务器文件全落错地方 | 用 `MSYS_NO_PATHCONV=1`，或 `printf '%s' '/path' \| gh secret set` 走 stdin |
| job outputs 含 secret | compose 报 `required variable IMAGE is missing` | 不跨 job 传镜像地址，deploy job 内用 env 自己拼 |
| bind mount 属主 | SQLite 报 `unable to open database file: out of memory (14)` | `chown -R 100:101 data logs`（`deploy.sh` 已自动做） |

> 完整版在 [memory/deploy-pipeline-gotchas.md](memory/deploy-pipeline-gotchas.md) 和
> [memory/windows-gitbash-pitfalls.md](memory/windows-gitbash-pitfalls.md)。

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


## License

Internal project.