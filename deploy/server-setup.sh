#!/usr/bin/env bash
# ==========================================================================
# 腾讯云服务器一次性初始化（新机器执行一次即可）
#
# 支持: Ubuntu 22.04+ / Debian 12+（腾讯云 Lighthouse 默认镜像就是 Ubuntu）
# 用法:
#   scp deploy/server-setup.sh user@server:/tmp/
#   ssh user@server 'sudo bash /tmp/server-setup.sh'
#
# 做的事:
#   1. 装 docker + docker compose 插件
#   2. 配置国内镜像加速（腾讯云内网地址，免流量费且快）
#   3. 建部署目录、准备 .env
#   4. 开放 8080 端口（云安全组需另行在控制台放行）
#
# 幂等：重复执行不会出错，已安装的部分会跳过。
# ==========================================================================

set -euo pipefail

# 仅支持 apt 系，避免在不支持的发行版上静默出错
if ! command -v apt-get >/dev/null 2>&1; then
  echo "!! 本脚本仅支持 Ubuntu/Debian（需要 apt-get）"
  echo "   当前系统: $(uname -s) $(uname -r)"
  exit 1
fi

if [ "$(id -u)" -ne 0 ]; then
  echo "!! 请用 root 运行: sudo bash $0"
  exit 1
fi

# 由 deploy.yml 部署时的路径约定保持一致
DEPLOY_PATH="${DEPLOY_PATH:-/opt/go-backend}"

echo "==> 1/4 安装 Docker"
if command -v docker >/dev/null 2>&1; then
  echo "    Docker 已安装: $(docker --version)"
else
  # 用阿里云的 apt 源（腾讯云官方 convenience script 也可，但国内源更稳）
  apt-get update -y
  apt-get install -y ca-certificates curl gnupg lsb-release
  install -m 0755 -d /etc/apt/keyrings
  curl -fsSL https://download.docker.com/linux/ubuntu/gpg \
    | gpg --dearmor -o /etc/apt/keyrings/docker.gpg
  chmod a+r /etc/apt/keyrings/docker.gpg
  echo \
    "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] \
    https://download.docker.com/linux/ubuntu \
    $(. /etc/os-release && echo "$VERSION_CODENAME") stable" \
    > /etc/apt/sources.list.d/docker.list
  apt-get update -y
  apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
  systemctl enable --now docker
  echo "    安装完成: $(docker --version)"
fi

echo "==> 2/4 配置镜像加速"
mkdir -p /etc/docker
# 腾讯云为每台 CVM/轻量提供内网加速地址，格式:
#   <地域>.mirror.ccs.tencentyun.com   例: ap-guangzhou.mirror.ccs.tencentyun.com
# 仅在同地域内网可用；跨地域或非腾讯云机器请删掉这一行。
REGISTRY_MIRROR="${REGISTRY_MIRROR:-https://ap-guangzhou.mirror.ccs.tencentyun.com}"
cat > /etc/docker/daemon.json <<EOF
{
  "registry-mirrors": ["${REGISTRY_MIRROR}"],
  "log-driver": "json-file",
  "log-opts": { "max-size": "10m", "max-file": "3" },
  "live-restore": true
}
EOF
# 重启使生效；live-restore 开着时重启 docker 不会杀掉运行中的容器
systemctl restart docker
echo "    镜像加速: ${REGISTRY_MIRROR}"

echo "==> 3/4 准备部署目录 ${DEPLOY_PATH}"
# 上传/数据库目录需要非 root 用户可写，但实际进程是容器里的 app(10001)，
# 这里只保证目录存在并归当前系统用户，避免后续权限错乱。
mkdir -p "${DEPLOY_PATH}"
chown "$(logname 2>/dev/null || echo root)":"$(logname 2>/dev/null || echo root)" "${DEPLOY_PATH}" 2>/dev/null || true

if [ ! -f "${DEPLOY_PATH}/.env" ]; then
  cat > "${DEPLOY_PATH}/.env" <<'EOF'
# 生产环境配置 —— 不要提交到仓库
# 生成随机 JWT 密钥: openssl rand -hex 32
JWT_SECRET=
APP_MODE=release
LOG_FORMAT=json
LOG_LEVEL=info
DB_DRIVER=sqlite
DB_DSN=./data/app.db
EOF
  echo "    已生成 ${DEPLOY_PATH}/.env，**请填入 JWT_SECRET**"
else
  echo "    .env 已存在，跳过（不覆盖）"
fi

echo "==> 4/4 防火墙（仅本机 ufw）"
if command -v ufw >/dev/null 2>&1; then
  # 只开 SSH 和应用端口。云安全组仍需在控制台单独放行 8080。
  ufw allow 22/tcp >/dev/null 2>&1 || true
  ufw allow 8080/tcp >/dev/null 2>&1 || true
  echo "    已放行 22, 8080（ufw 未启用则无实际影响）"
fi

cat <<EOF

==========================================
初始化完成。接下来：

  1. 填写密钥（务必）:
       nano ${DEPLOY_PATH}/.env
       # JWT_SECRET=$(openssl rand -hex 32)

  2. 确认云安全组放行 8080（腾讯云控制台，脚本管不到）

  3. 配置 GitHub Secrets:
       TCR_REGISTRY     ccr.ccs.tencentyun.com/<命名空间>
       TCR_IMAGE        go-backend
       TCR_USERNAME     TCR 访问凭证用户名
       TCR_PASSWORD     TCR 访问凭证密码
       DEPLOY_HOST      $(hostname -I | awk '{print $1}')
       DEPLOY_USER      $(logname 2>/dev/null || echo root)
       DEPLOY_SSH_KEY   专用私钥全文
       DEPLOY_PATH      ${DEPLOY_PATH}

  4. 把本仓库加入 TCR 可信来源（同地域内网免登录）:
       docker login ccr.ccs.tencentyun.com -u <用户名> -p <密码>
==========================================
EOF
