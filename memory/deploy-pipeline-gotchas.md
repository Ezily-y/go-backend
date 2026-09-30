---
name: deploy-pipeline-gotchas
description: go-backend CI/CD 部署链路的三个隐蔽失败模式及根因
metadata:
  type: project
---

go-backend 的部署链是 push main → ci.yml 质量门禁 → deploy.yml（推 TCR → SSH →
docker compose up）。踩过的三个隐蔽坑，排查时按这个顺序看：

**1. workflow 里内联 `ssh "整段脚本"` 会在 runner 本地就炸。** 双引号内 bash 仍做
命令替换与语法解析：`$(seq 1 30)` 被本地展开、`"${i}"` 在 `set -u` 下变 unbound、
括号与注释随上下文漂移，报 `syntax error near unexpected token '('` —— 命令压根没到
服务器。根治法是把远端逻辑拆成 `deploy/deploy.sh`，workflow 只做 scp 下发 + ssh 执行。

**2. job outputs 一旦含 secret 就会被 GitHub 整条清空。** 曾把镜像地址
`registry/repo:sha-xxx` 通过 `needs.*.outputs.image` 传给 deploy job，因为
`TCR_REGISTRY` 是 secret，输出被清成空串，compose 报 `required variable IMAGE is
missing`。修法是不跨 job 传该值，deploy job 内用 job-level env 自己拼（`github.sha`
前 7 位与构建侧一致）。注意 `steps.meta` 是 metadata-action，本身也没有 image 输出。

**3. bind mount 目录属主必须匹配容器内 uid。** 宿主 `./data` 由 docker 按 root 建，
镜像跑的是 `adduser -S` 的非 root（uid=100），SQLite 打不开，日志显示
`unable to open database file: out of memory (14)` —— 那是 SQLITE_CANTOPEN，与内存
无关。进程 fatal 后被 restart 策略不停拉起，健康检查 30s 超时。修法：启动前
`chown -R 100:101 data logs`，`.env` 给 0400 并改属主。

服务是否真的起来，看 `docker compose ps` 的 `(healthy)` 和 `/health` 返回
`database: up`。**健康检查路径是 `/health` 不是 `/healthz`。**

相关：[[windows-gitbash-pitfalls]]
