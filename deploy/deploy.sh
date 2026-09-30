#!/usr/bin/env bash
# ==========================================================================
# 生产部署脚本（在服务器上执行，由 deploy.yml 通过 scp 下发）
#
# 参数（按位置）：
#   $1  DEPLOY_PATH   部署目录
#   $2  IMAGE         要部署的完整镜像地址 <registry>/<repo>:<tag>
#   $3  TCR_REGISTRY  镜像仓库地址
#   $4  TCR_USERNAME  镜像仓库用户名
#   $5  TCR_PASSWORD  镜像仓库密码
#
# 为什么是独立脚本而不是内联在 workflow 的 run 块里：
#   内联写法要把整段脚本塞进 ssh 的双引号参数，本地 bash 会先解析它 ——
#   $(seq ...) 被本地展开、"${i}" 在 set -u 下变 unbound、括号与注释的
#   处理随上下文漂移，最后在 runner 本地就报 syntax error，命令压根
#   到不了服务器。独立文件则完全规避这类引号/转义问题。
# ==========================================================================

set -euo pipefail

if [ "$#" -ne 5 ]; then
  echo "!! 用法: $0 <DEPLOY_PATH> <IMAGE> <TCR_REGISTRY> <TCR_USERNAME> <TCR_PASSWORD>" >&2
  exit 2
fi

DEPLOY_PATH="$1"
IMAGE="$2"
TCR_REGISTRY="$3"
TCR_USERNAME="$4"
TCR_PASSWORD="$5"

cd "${DEPLOY_PATH}"

# 首次部署时把生产 compose 模板准备好
if [ ! -f compose.prod.yaml ]; then
  echo '!! 未找到 compose.prod.yaml，请先完成服务器初始化'
  exit 1
fi

# .env 里存 JWT_SECRET 等，不该被 CI 覆盖，缺失则报错而不是用默认值
if [ ! -f .env ]; then
  echo '!! 未找到 .env，请先执行: cp .env.example .env 并填写 JWT_SECRET'
  exit 1
fi

# 拦截样例默认密钥：dev-only-secret-change-me-0123456789 长度 36，
# 能通过 config.Validate 的 >=32 校验，但它是公开已知值 ——
# 用它签的 JWT 任何人可伪造。直接 cp .env.example 不改就部署是典型事故。
if grep -qE '^JWT_SECRET=(dev-only-secret-change-me|change-me)' .env; then
  echo '!! .env 中的 JWT_SECRET 仍是样例默认值，拒绝部署。'
  echo '   请生成随机值: openssl rand -hex 32'
  exit 1
fi

# 登录 TCR（服务器侧也要认证才能拉私有镜像）
docker login "${TCR_REGISTRY}" \
  -u "${TCR_USERNAME}" \
  -p "${TCR_PASSWORD}"

# IMAGE 供 compose.prod.yaml 的 ${IMAGE:?...} 取值
export IMAGE

echo "==> 拉取新镜像: ${IMAGE}"
docker compose -f compose.prod.yaml pull app

# ---------------------------------------------------------------------------
# 修正宿主 bind mount 的属主
#
# compose.prod.yaml 把 ./data 和 ./logs 挂进容器。这两个目录由 docker 在
# 宿主机上按 root 创建，而镜像里跑的是非 root 用户（busybox adduser -S 分到
# uid=100），于是容器内 SQLite 直接打不开：
#   初始化应用失败: 连接数据库失败: unable to open database file: out of memory (14)
# "out of memory (14)" 是 SQLITE_CANTOPEN，跟内存无关，纯粹是目录不可写。
#
# 同理 .env 由 root 拥有、600 权限，容器内的 app 用户读不到 JWT_SECRET，
# config.Validate() 会因密钥缺失/过短拒绝启动。统一按镜像里的 uid 修一次。
# uid/gid 从镜像里现取，避免 Dockerfile 改了 adduser 之后这里悄悄失效。
# ---------------------------------------------------------------------------
echo "==> 修正挂载目录属主（.env、data、logs）"
# 容器内是非 root 用户（Dockerfile 的 `adduser -S app`），宿主目录却是 root 拥有，
# 容器一旦挂上去就打不开 SQLite / 读不到 .env。按镜像里的 uid 修一次即可。
# 这里不写 `docker run --entrypoint id`：-u 会被 docker 自己的 --user 吃掉，
# 导致参数错位。alpine/busybox 的 `adduser -S` 首个系统用户固定是 uid=100，
# 与镜像实测一致（`id` 输出 uid=100 gid=101）；Dockerfile 改动时同步核对这里。
CONTAINER_UID=100
CONTAINER_GID=101

mkdir -p data logs
chown -R "${CONTAINER_UID}:${CONTAINER_GID}" data logs
# .env 要能被容器内用户读取，但不能让宿主机其它用户看到密钥
chown "${CONTAINER_UID}:${CONTAINER_GID}" .env
chmod 0400 .env
echo "    data/logs/.env 属主已设为 ${CONTAINER_UID}:${CONTAINER_GID}"

echo '==> 启动新版本'
docker compose -f compose.prod.yaml up -d

echo '==> 等待健康检查'
for i in $(seq 1 30); do
  if curl -fsS http://127.0.0.1:8080/health >/dev/null 2>&1; then
    echo "健康检查通过 (等待 ${i}s)"
    docker compose -f compose.prod.yaml ps
    exit 0
  fi
  sleep 1
done

echo '!! 健康检查超时，回滚到上一版'
docker compose -f compose.prod.yaml logs --tail=50 app
exit 1
