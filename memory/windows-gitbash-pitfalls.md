---
name: windows-gitbash-pitfalls
description: go-backend 项目在 Windows + Git Bash 环境下会踩的路径转换与工具链坑
metadata:
  type: project
---

go-backend 本机是 Windows 11 + Git Bash，有三个反复踩到的坑：

**MSYS 路径转换会改写传给 SSH 的绝对路径。** `/opt/go-backend` 从 Git Bash 传给
`ssh`/`gh` 会被改写成 `D:/MySoftwares/Git/opt/go-backend`。曾导致 GitHub Secret
`DEPLOY_PATH` 被存成带盘符的错值，服务器上文件全落到 `/root/D:/...` 而真正的
`/opt/go-backend/.env` 从未被读到。修法：用 `MSYS_NO_PATHCONV=1` 前缀，或
`printf '%s' '/path' | gh secret set NAME` 走 stdin 绕开转换。

**`docker run --entrypoint id -u` 取镜像内 uid 不可用。** `-u` 会被 docker 自己的
`--user` 选项吃掉，参数错位后报 "requires at least 1 argument" 或去拉 `app:latest`。
镜像内 uid/gid 直接写常量（实测 `adduser -S` 的首个系统用户是 `uid=100 gid=101`），
Dockerfile 改动时同步核对。

**本机没装 make 和 golangci-lint。** 两者都由 CI 执行，本地直接敲原始 go 命令
（`go build ./...`、`go vet ./...`、`go test ./...`、`gofmt -l .`）。验证 YAML 时
`open(path)` 在 Windows 默认 GBK 会炸，要显式 `encoding='utf-8'`。

相关：[[deploy-pipeline-gotchas]]
