// Package bootstrap 组装应用依赖（依赖注入），并在首次启动时写入种子数据。
//
// 项目规模不大，手写装配即可，不必引入 wire / dig 这类框架：
// 依赖关系一眼可见，编译期就能发现漏传。
package bootstrap

import (
	"context"
	"errors"
	"fmt"

	"github.com/gin-gonic/gin"

	"go-backend/internal/core/apperr"
	"go-backend/internal/auth"
	"go-backend/internal/core/config"
	"go-backend/internal/core/database"
	"go-backend/internal/handler"
	"go-backend/internal/core/logger"
	"go-backend/internal/model"
	"go-backend/internal/repository"
	"go-backend/internal/router"
	"go-backend/internal/service"
)

// App 持有运行期需要显式关闭的资源。
type App struct {
	Cfg    *config.Config
	Engine *gin.Engine // 已注册全部路由的引擎
}

// New 装配全部依赖并注册路由。
func New(cfg *config.Config) (*App, error) {
	// ---- 1. 基础设施 ----
	if err := database.Connect(cfg.Database); err != nil {
		return nil, err
	}

	jwtSvc, err := auth.New(
		cfg.App.JWT.Secret,
		cfg.App.JWT.AccessTTL,
		cfg.App.JWT.RefreshTTL,
	)
	if err != nil {
		return nil, fmt.Errorf("初始化认证服务失败: %w", err)
	}

	// ---- 2. 仓储层（传 nil 表示使用 database.Get() 的全局连接）----
	userRepo := repository.NewUserRepo(nil)
	apiKeyRepo := repository.NewAPIKeyRepo(nil)

	// ---- 3. 服务层 ----
	userSvc := service.NewUserService(userRepo, jwtSvc)
	apiKeySvc := service.NewAPIKeyService(apiKeyRepo)

	// ---- 4. 处理层 ----
	h := &router.Handlers{
		Auth:   handler.NewAuthHandler(userSvc),
		User:   handler.NewUserHandler(userSvc),
		APIKey: handler.NewAPIKeyHandler(apiKeySvc),
		System: handler.NewSystemHandler(),
		JWT:    jwtSvc,
		KeySVC: apiKeySvc,
	}

	// ---- 5. 首次启动种子数据 ----
	if err := seed(cfg, userRepo); err != nil {
		return nil, err
	}

	return &App{Cfg: cfg, Engine: router.New(cfg, h)}, nil
}

// seed 首次启动时创建管理员账号。
// 已存在任何用户时跳过，避免每次启动都用配置里的密码覆盖已有数据。
func seed(cfg *config.Config, repo repository.UserRepository) error {
	if !cfg.App.Bootstrap.Enabled {
		return nil
	}

	ctx := context.Background()

	// 直接按配置的用户名查，比「表非空就跳过」更直观：
	// 即使表里有其他用户，管理员账号缺失时仍会补建。
	if _, err := repo.GetByUsername(ctx, cfg.App.Bootstrap.Username); err == nil {
		return nil
	} else if !errors.Is(err, repository.ErrNotFound) {
		return err
	}

	b := cfg.App.Bootstrap
	hash, err := auth.HashPassword(b.Password)
	if err != nil {
		return fmt.Errorf("初始化管理员密码失败: %w", err)
	}

	admin := &model.User{
		Username:     b.Username,
		PasswordHash: hash,
		Nickname:     "超级管理员",
		Role:         model.Role(b.Role),
		Status:       1,
	}
	if err := repo.Create(ctx, admin); err != nil {
		// 并发启动时可能被另一实例抢先创建，唯一索引冲突不算失败。
		if !errors.Is(err, errUserExists) {
			return err
		}
		return nil
	}

	// 仅在首次初始化时打印一次凭据提示，方便本地开发。
	if cfg.App.Mode != "release" {
		logger.Warnf("已创建初始管理员账号: %s / %s（请立即修改密码）", b.Username, b.Password)
	}
	return nil
}

// errUserExists 是 repository 在用户名冲突时返回的业务错误，
// 用 errors.Is 匹配，避免依赖具体错误文本。
var errUserExists = apperr.New(apperr.CodeUserExists, "用户名已存在")
