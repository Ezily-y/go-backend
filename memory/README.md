# go-backend 踩坑记录

> 这个目录是**项目自带的踩坑记忆**，跟着仓库一起 clone / 复制走。
> 新的踩坑经验一律写进这里，不要只留在本机的 Claude 记忆目录里 —— 那个目录不进 git，换机器就没了。
>
> 每条记录一个主题一个文件，格式：
>
> ```markdown
> ---
> name: <kebab-case 短名>
> description: 一句话说明，便于检索
> metadata:
>   type: project
> ---
>
> 现象 → 根因 → 修法（写清楚"为什么"，否则下次还是踩）
> ```
>
> 文件之间用 `[[文件名]]` 互链。

## 索引

- [Windows / Git Bash 坑](windows-gitbash-pitfalls.md) — MSYS 路径转换改写 SSH 参数、`docker -u` 吃参数、本机无 make/golangci-lint
- [部署链路坑](deploy-pipeline-gotchas.md) — 内联 ssh 脚本本地炸、job outputs 含 secret 被清空、bind mount 属主导致 SQLite 打不开

## 用法

```bash
# 新增一条
cat > memory/<your-topic>.md <<'EOF'
---
name: your-topic
description: 一句话
metadata:
  type: project
---

现象 → 根因 → 修法
EOF
# 然后在本文件「索引」里加一行链接
```
