// Package database 负责建立数据库连接、执行自动迁移与生命周期管理。
//
// SQLite 驱动选用 github.com/glebarez/sqlite（纯 Go 实现），
// 而非 gorm.io/driver/sqlite（依赖 CGO）：免去交叉编译时配置 C 工具链的麻烦，
// 部署到 Linux 服务器只需 GOOS=linux go build 即可。
//
// MVP 阶段使用 GORM 自动迁移（AutoMigrate）；
// 进入需要严格版本控制的阶段后，再引入 golang-migrate 的 migrations/ 目录。
package database

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	sqlite "github.com/glebarez/sqlite"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"go-backend/internal/config"
	"go-backend/internal/logger"
	"go-backend/internal/model"
)

// db 全局数据库句柄；raw 保留底层连接，用于健康检查与优雅关闭。
var (
	db  *gorm.DB
	raw *sql.DB
)

// Connect 根据配置建立连接并完成表结构迁移。
func Connect(cfg config.DatabaseConfig) error {
	var dialector gorm.Dialector

	switch cfg.Driver {
	case "sqlite":
		// SQLite 要求数据目录必须存在，否则连接直接失败。
		if dir := filepath.Dir(cfg.DSN); dir != "" && dir != "." {
			if err := os.MkdirAll(dir, 0o750); err != nil {
				return fmt.Errorf("创建 SQLite 目录失败: %w", err)
			}
		}
		dialector = sqlite.Open(cfg.DSN + sqliteParams())
	case "postgres":
		dialector = postgres.Open(cfg.DSN)
	default:
		return fmt.Errorf("不支持的数据库驱动: %s", cfg.Driver)
	}

	var err error
	db, err = gorm.Open(dialector, &gorm.Config{
		Logger:         newGormLogger(cfg.SlowThreshold),
		PrepareStmt:    true, // 预编译语句，减少重复解析开销
		TranslateError: true, // 把驱动错误翻译成 gorm.ErrDuplicatedKey 等标准错误
	})
	if err != nil {
		return fmt.Errorf("连接数据库失败: %w", err)
	}

	// 连接池调优：上限防止打爆数据库，空闲池避免频繁建连。
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("获取底层连接失败: %w", err)
	}
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	sqlDB.SetConnMaxIdleTime(10 * time.Minute)
	raw = sqlDB

	// 连通性验证，失败时尽早暴露问题。
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := sqlDB.PingContext(ctx); err != nil {
		return fmt.Errorf("数据库连通性检查失败: %w", err)
	}

	return Migrate()
}

// sqliteParams 附加 SQLite 连接参数。
// WAL 模式让读写并发大幅提升；busy_timeout 避免写锁竞争直接报错。
func sqliteParams() string {
	return "?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=on&_synchronous=NORMAL"
}

// Migrate 执行自动迁移。
// MVP 用 AutoMigrate 快速迭代；表结构复杂后应切换到版本化迁移脚本。
func Migrate() error {
	if db == nil {
		return fmt.Errorf("数据库尚未初始化")
	}
	targets := []interface{}{
		&model.User{},
		&model.APIKey{},
		&model.APILog{},
	}
	if err := db.AutoMigrate(targets...); err != nil {
		return fmt.Errorf("自动迁移失败: %w", err)
	}
	logger.Debugf("数据库迁移完成，共 %d 张表", len(targets))
	return nil
}

// Get 返回全局 *gorm.DB。
func Get() *gorm.DB {
	if db == nil {
		panic("数据库尚未初始化，请先调用 database.Connect")
	}
	return db
}

// Ping 校验数据库连通性，供健康检查接口调用。
func Ping(ctx context.Context) error {
	if raw == nil {
		return fmt.Errorf("数据库尚未初始化")
	}
	return raw.PingContext(ctx)
}

// Close 关闭连接池。
func Close() error {
	if raw == nil {
		return nil
	}
	return raw.Close()
}

// newGormLogger 构造 GORM 日志器。
// 直接使用自定义 gormBridge，把 SQL 日志桥接到 zap，
// 慢查询阈值由桥接层内部判断并抬升到 Warn 级别。
func newGormLogger(threshold time.Duration) gormlogger.Interface {
	if threshold <= 0 {
		threshold = 200 * time.Millisecond
	}
	return gormBridge{slow: threshold}
}

// gormBridge 实现 gorm/logger.Interface，把日志写入 zap。
type gormBridge struct {
	slow time.Duration // 慢查询阈值，超过则以 Warn 级别记录
}

// Trace 记录每条 SQL 的耗时；错误与慢查询分别落到 Error/Warn。
func (b gormBridge) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	elapsed := time.Since(begin)
	query, rows := fc()
	switch {
	case err != nil:
		logger.Errorf("[SQL] %v | %s | rows=%d | %s", err, query, rows, elapsed)
	case elapsed > b.slow:
		logger.Warnf("[SQL] 慢查询 %s | %s | rows=%d", elapsed, query, rows)
	default:
		logger.Debugf("[SQL] %s | rows=%d | %s", query, rows, elapsed)
	}
}

// LogMode 返回自身，表明日志级别由 zap 侧统一决定。
func (b gormBridge) LogMode(gormlogger.LogLevel) gormlogger.Interface { return b }

func (gormBridge) Info(_ context.Context, msg string, data ...interface{}) {
	logger.Debugf("[SQL] %s %s", msg, fmt.Sprint(data...))
}

func (gormBridge) Warn(_ context.Context, msg string, data ...interface{}) {
	logger.Warnf("[SQL] %s %s", msg, strings.TrimSpace(fmt.Sprint(data...)))
}

func (gormBridge) Error(_ context.Context, msg string, data ...interface{}) {
	logger.Errorf("[SQL] %s %s", msg, fmt.Sprint(data...))
}
