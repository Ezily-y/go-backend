---
name: go-backend-deploy
description: 在 go-backend 项目里执行部署/CI 排查。当用户要求"部署""发布""CI 挂了""镜像推不上去""腾讯云服务器"时使用。
---

# go-backend 部署

## 前置确认（先做，别跳）

1. 检查部署相关文件是否齐全：
   ```bash
   ls -la .github/workflows/ deploy/ Dockerfile 2>&1
   ```
   应能看到 `.github/workflows/ci.yml` 与 `.github/workflows/deploy.yml`，以及 `deploy/` 目录下的部署脚本。
2. 本地 docker daemon 是否启动：`docker info`。失败就是 Docker Desktop 没开，提示用户先启动它，再继续。
3. 当前分支与远端：`git status`、`git remote -v`。

## 部署链路

```
git push origin main
        │
        ▼
.github/workflows/ci.yml      门禁：gofmt -l / go vet / go test
        │  通过
        ▼
.github/workflows/deploy.yml  ① docker build 打镜像
                              ② docker login 腾讯云 TCR
                              ③ docker push <TCR_REGISTRY>/<repo>:<tag>
                              ④ SSH 到腾讯云服务器
                              ⑤ scp 下发 compose.prod.yaml，注入 IMAGE 环境变量
                              ⑥ docker compose pull && docker compose up -d
                              ⑦ curl /health 校验
        │
        ▼
腾讯云服务器上的 `docker compose -f compose.prod.yaml up -d` 起 app 容器
```

tag 约定：`sha-<7位短哈希>`（如 `sha-a1b2c3d`），由 `docker/metadata-action` 的 `type=sha,prefix=sha-` 生成，**不要只用 `latest`**。

## GitHub Secrets 清单

仓库 Settings → Secrets and variables → Actions → New repository secret。
**名字必须完全一致**，`deploy.yml` 就是按下面这些名字引用的。

| Secret | 内容 | 从哪拿 |
|---|---|---|
| `TCR_REGISTRY` | 镜像仓库地址，如 `ccr.ccs.tencentyun.com/<namespace>` | 腾讯云控制台 → 容器镜像服务 TCR → 镜像仓库 → "公网访问地址"。**不带 `https://`、不带尾部 `/`** |
| `TCR_IMAGE` | 镜像名，如 `go-backend` | 同上；最终镜像 = `${TCR_REGISTRY}/${TCR_IMAGE}:sha-<短哈希>` |
| `TCR_USERNAME` | TCR 登录用户名 | 同上，"访问凭证"页的用户名 |
| `TCR_PASSWORD` | TCR 登录密码 | 同上，同页密码（**不是**腾讯云账号密码，也**不是** SecretId/SecretKey） |
| `DEPLOY_HOST` | 服务器公网 IP 或域名 | 腾讯云控制台 → 轻量应用服务器/CVM → 公网 IP |
| `DEPLOY_USER` | SSH 登录用户名 | 通常 `ubuntu`（轻量）或 `root` |
| `DEPLOY_SSH_KEY` | 私钥**全文** | 本机 `cat ~/.ssh/id_ed25519`（私钥，不是 `.pub`）。没有先 `ssh-keygen -t ed25519` |
| `DEPLOY_PATH` | 服务器部署目录 | 一般 `/opt/go-backend`；`server-setup.sh` 默认也是这个值 |

**`JWT_SECRET` 不是 GitHub Secret** —— 它写在服务器的 `${DEPLOY_PATH}/.env` 里
（由 `deploy/server-setup.sh` 生成模板）。CI 不持有、也读不到应用密钥。
注意 `.env.example` 里的样例值 `dev-only-secret-change-me-0123456789` 长度 36，
能通过 `>=32` 校验但属公开已知值；**`deploy.yml` 会检测到并拒绝部署**。
生成真值：`openssl rand -hex 32`。

把公钥装到服务器：
```bash
ssh-copy-id -i ~/.ssh/id_ed25519.pub <DEPLOY_USER>@<DEPLOY_HOST>
```

## 常见故障排查

### 1. 镜像推不上去

```bash
# 本地先手动验登录（把变量换成真实值，密码走 stdin 不要落 shell history）
echo "$TCR_PASSWORD" | docker login "$TCR_REGISTRY" --username "$TCR_USERNAME" --password-stdin
docker push "$TCR_REGISTRY/go-backend:<tag>"
```
- 腾讯云 TCR 使用固定用户名登录：`docker login ccr.ccs.tencentyun.com --username=100038334149`，密码为 TCR 访问凭证中设置的固定密码。
- 报 `unauthorized` / `authentication required` → Secret 名字拼错，或 `TCR_PASSWORD` 填成了腾讯云账号密码（要用 TCR 的"访问凭证"）。
- 报 `repository does not exist` → TCR 控制台里要先**创建同名命名空间和仓库**，或开"自动创建仓库"。
- 地址格式：`ccr.ccs.tencentyun.com/<namespace>/<repo>`，**不带协议、不带尾斜杠**，多了少了都报错。
- 用 GitHub Actions 从公网推 TCR：确认 TCR 实例开了"公网访问"，或 runner 白名单在腾讯云网段内。

### 2. SSH 连不上

```bash
ssh -v -i ~/.ssh/id_ed25519 <DEPLOY_USER>@<DEPLOY_HOST> 'echo ok'
```
- `Permission denied (publickey)` → 服务器 `authorized_keys` 里没有这把公钥；或私钥文件换行在 Secrets 里被破坏。上传 Secret 时保持单行 base64 完整。
- `Connection timed out` → 服务器安全组/防火墙没放行 22 端口（腾讯云控制台 → 安全组 → 入站规则）。
- `Host key verification failed` → workflow 里要先写 known_hosts：
  ```bash
  mkdir -p ~/.ssh && ssh-keyscan -H "$DEPLOY_HOST" >> ~/.ssh/known_hosts
  ```
  或临时 `StrictHostKeyChecking=accept-new`。
- Windows 本机手动测：Git Bash 下路径写 `~/.ssh/id_ed25519`，别写 `C:\...`。

### 3. 服务起不来

```bash
ssh <DEPLOY_USER>@<DEPLOY_HOST>
cd <DEPLOY_PATH>
# 服务器上生产用 compose.prod.yaml（不在 Docker Compose 自动发现列表里，必须 -f）
docker compose -f compose.prod.yaml logs --tail=200 app
docker compose -f compose.prod.yaml ps
```
- 启动即退出 + 日志里 `app.jwt.secret 长度必须 >= 32` → **`JWT_SECRET` 没配或太短**。生成一个：
  ```bash
  openssl rand -base64 48   # 64 字符，远超 32
  ```
  写进服务器 `.env`（compose 里 `JWT_SECRET=${JWT_SECRET}`）或 GitHub Secret，然后 `docker compose up -d`。
- 报配置非法 `app.mode` / `log.level` / `database.driver` → 对应值不在白名单里，见 `config/config.yaml` 的注释。
- 端口冲突 `bind: address already in use` → 宿主机 8080 被占：`ss -lntp | grep 8080`。
- 数据库连不上 → `database.dsn` 路径目录是否存在（sqlite 要 `./data/`）；postgres 要检查 `DB_DSN` 与容器网络。

健康检查：
```bash
curl -fsS http://127.0.0.1:8080/health && echo OK
# 远程：
curl -fsS http://<DEPLOY_HOST>:8080/health && echo OK   # 需安全组放行 8080
```

### 4. CI 挂了

先看是哪一步挂的：
```bash
gh run list --branch main --limit 5
gh run view <run-id> --log-failed
```
- `gofmt -l` 有输出 → 本地跑 `gofmt -w .` 再提交。
- `go vet` / `go test` 失败 → 本地复现 `go vet ./... && go test ./...`。
- `go mod tidy` 相关失败 → 本地 `go mod tidy` 提交 `go.mod`/`go.sum`。

### 5. 本地先验证（推荐在推送前做）

```bash
# docker daemon 未启动时，先打开 Docker Desktop
docker info || echo "请先启动 Docker Desktop"

# 开发环境：根目录 docker-compose.yaml（postgres + redis + 本地 build）
# 注意别误用 compose.prod.yaml —— 那是生产编排，会去 TCR 拉镜像
docker compose build
docker compose up -d
docker compose logs -f app
curl -fsS http://127.0.0.1:8080/health

# 生产编排本地试跑（需要 TCR 登录 + 设置 IMAGE）
# IMAGE=ccr.ccs.tencentyun.com/<ns>/go-backend:sha-xxx docker compose -f compose.prod.yaml up -d
```
**两套 compose 文件不要搞混**：`docker-compose.yaml` = 开发（默认），
`compose.prod.yaml` = 生产（必须显式 `-f`）。
（上面 compose 命令已在 `.claude/settings.local.json` 中放行 `Bash(docker *)`，不弹权限框。）

## 回滚

**回滚镜像（最快，不动 git）：**
```bash
ssh <DEPLOY_USER>@<DEPLOY_HOST>
cd <DEPLOY_PATH>
# 看历史 tag（sha- 开头的就是可用的回滚点）
docker images --format '{{.Repository}}:{{.Tag}}' | grep go-backend

# 用上一个 sha 标签重新起（compose.prod.yaml 的 image 由 IMAGE 环境变量注入）
export IMAGE=<上一个 sha-xxx>
docker compose -f compose.prod.yaml pull app
docker compose -f compose.prod.yaml up -d

# 想让回滚长期生效：把 IMAGE 写进 .env，否则重启容器会回落
sed -i "s|^IMAGE=.*|IMAGE=${IMAGE}|" .env
```

**回滚代码（要重新构建）：**
```bash
git log --oneline -10            # 找到好的 commit
git revert <bad-sha>             # 用 revert 不用 reset，保留历史
git push origin main             # 触发 CI/CD 重新部署
```

**回滚后必查：** `docker compose -f compose.prod.yaml logs --tail=200 app` + `curl /health`。

## 禁止事项

- **不要把任何密钥写进仓库** —— 密钥只进 GitHub Secrets 或服务器 `.env`（两者都已 gitignore）。
- **不要 `docker push :latest` 到生产** —— 生产用 sha tag；`latest` 无法回滚到确定版本。
- **不要 `git push --force` 到 `main`**（已在项目级权限 `deny` 里拦掉）。
- **不要把 `JWT_SECRET` 写进 `config/config.yaml`** —— 该文件会被提交。
- 不要在日志里打印 DSN、密钥、token；`internal/core/response` 的 5xx 已经只对外给通用文案，别在 handler 里自己泄漏。
- 不要在 `.claude/settings.json` 的 `env` 字段里放任何真实凭证。
