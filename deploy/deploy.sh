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
