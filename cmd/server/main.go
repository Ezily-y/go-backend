// Go 后端服务平台入口。
//
// 启动流程：
//
//	加载配置 → 初始化日志 → 装配依赖（DB/仓储/服务/路由） → 启动 HTTP → 等待退出信号
//
// 退出时按相反顺序关闭资源，保证在途请求处理完毕后再断数据库连接。
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go-backend/internal/bootstrap"
	"go-backend/internal/config"
	"go-backend/internal/database"
	"go-backend/internal/logger"
	"go-backend/internal/version"
)

// 优雅退出时等待在途请求完成的最长时间。
const shutdownTimeout = 10 * time.Second

func main() {
	os.Exit(run())
}

// run 执行完整的应用生命周期，返回退出码（0=正常，1=失败）。
// 拆分为独立函数是为了让 defer 在 os.Exit 之前执行，避免资源泄漏。
func run() int {
	// 支持 -c / -config 指定配置文件路径，便于多环境部署。
	var (
		configPath string
		showVer    bool
	)
	flag.StringVar(&configPath, "c", "", "配置文件路径（默认 config/config.yaml）")
	flag.StringVar(&configPath, "config", "", "配置文件路径（同 -c）")
	flag.BoolVar(&showVer, "v", false, "打印版本信息后退出")
	flag.Parse()

	if showVer {
		fmt.Println(version.Get())
		return 0
	}

	// 1. 配置：失败直接退出，此时日志尚未初始化，只能打印到 stderr。
	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[FATAL] 加载配置失败: %v\n", err)
		return 1
	}

	// 2. 日志：之后的所有错误都走 zap。
	if err = logger.Initialize(logger.Options{
		Level:  cfg.Log.Level,
		Format: cfg.Log.Format,
		File:   cfg.Log.File,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "[FATAL] 初始化日志失败: %v\n", err)
		return 1
	}
	defer logger.Sync()

	logger.Infof("启动 %s v%s | mode=%s | addr=%s", cfg.App.Name, version.GetRaw(), cfg.App.Mode, cfg.Addr())

	// 3. 装配依赖 + 注册路由。
	app, err := bootstrap.New(cfg)
	if err != nil {
		logger.Fatalf("初始化应用失败: %v", err)
	}

	// 4. 启动 HTTP 服务（异步），主协程阻塞等待退出信号。
	srv := &http.Server{
		Addr:              cfg.Addr(),
		Handler:           app.Engine,
		ReadHeaderTimeout: 10 * time.Second, // 防 Slowloris 攻击
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20, // 1 MB
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Infof("HTTP 服务已就绪，监听 %s", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	// 5. 同时监听中断信号与启动错误，谁先到听谁的。
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case startErr := <-errCh:
		logger.Errorf("HTTP 服务启动失败: %v", startErr)
		return 1
	case sig := <-quit:
		logger.Infof("收到退出信号 %v，开始优雅关闭...", sig)
	}

	// 6. 优雅关闭：先停止接收新请求，等待在途请求完成，再释放资源。
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Errorf("HTTP 服务关闭超时: %v", err)
	}
	if err := database.Close(); err != nil {
		logger.Errorf("关闭数据库连接失败: %v", err)
	}
	logger.Infof("已安全退出")
	return 0
}
