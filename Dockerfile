# ==========================================================================
# 多阶段构建：编译阶段用完整 Go 工具链，运行阶段只留二进制
# 最终镜像基于 alpine，体积可控制在 30MB 以内。
# ==========================================================================

# ---- 编译阶段 ----
# 版本必须与 go.mod 的 go 指令一致（当前 go 1.26.0）。
# 用 1.25 会直接失败: "go.mod requires go >= 1.26.0"（已实测）。
FROM golang:1.26-alpine AS builder

# 国内构建走 goproxy.cn。不开启的话 go mod download 会超时或极慢，
# 而 GOTOOLCHAIN 自动下载工具链（若 go.mod 比镜像新）也依赖此代理。
ENV GOPROXY=https://goproxy.cn,direct

WORKDIR /build

# 先只拷贝依赖清单，利用 Docker 层缓存：
# 只要 go.mod/go.sum 未变，重新构建时不会重下依赖。
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# 静态编译，产出不依赖 libc 的单文件二进制。
# CGO_ENABLED=0 是必须的：SQLite 驱动走纯 Go 实现，无需 C 工具链。
ARG VERSION=dev
ARG COMMIT=none
ARG BUILD_TIME=unknown
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags "-s -w \
      -X 'go-backend/internal/version.Version=${VERSION}' \
      -X 'go-backend/internal/version.Commit=${COMMIT}' \
      -X 'go-backend/internal/version.BuildTime=${BUILD_TIME}'" \
    -o /build/server ./cmd/server

# ---- 运行阶段 ----
FROM alpine:3.20

# ca-certificates：调用第三方 HTTPS API 必需
# tzdata：容器内正确解析时区
RUN apk add --no-cache ca-certificates tzdata && \
    addgroup -S app && adduser -S -G app app

ENV TZ=Asia/Shanghai

WORKDIR /app

# 以非 root 用户运行，降低容器逃逸后的影响面
COPY --from=builder /build/server /app/server
COPY --from=builder /build/config/config.yaml /app/config/config.yaml

# 数据与日志目录需要写权限（SQLite 文件、上传文件、日志落盘）
RUN mkdir -p /app/data/uploads /app/logs && chown -R app:app /app

USER app

EXPOSE 8080

# 健康检查路径是 /health（与 compose.prod.yaml 保持一致）。
# 路由定义见 internal/router/router.go: e.GET("/health", h.System.Health)
# alpine 自带 busybox wget，无需安装 curl。
# 注意不是 /healthz —— 实测该路径返回 404。
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
    CMD wget -qO- http://127.0.0.1:8080/health || exit 1

ENTRYPOINT ["/app/server"]
CMD ["-c", "/app/config/config.yaml"]
