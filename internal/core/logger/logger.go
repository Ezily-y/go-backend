// Package logger 封装 zap 结构化日志，全局提供统一的 logger 实例。
//
// 设计要点：
//   - 使用 uber-go/zap，兼顾性能与书写便利；
//   - 启动时由 Initialize 初始化一次，之后全站通过包级函数共享同一实例；
//   - 未初始化前默认使用 no-op logger，保证任何阶段调用都不会 panic；
//   - 支持同时输出到控制台与文件（按大小滚动），便于本地调试与服务器留档。
package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// global 全局日志句柄，进程内共享一份。
// 初始值为 no-op，避免 Initialize 之前调用导致空指针 panic。
var global = &Logger{SugaredLogger: zap.NewNop().Sugar()}

// Logger 全局日志句柄。
type Logger struct {
	*zap.SugaredLogger
	zap *zap.Logger // 原始 logger，供需要强类型字段的场景使用
}

// Options 日志初始化选项。
type Options struct {
	Level  string // debug | info | warn | error
	Format string // console | json
	File   string // 空表示不写文件
}

// Initialize 初始化全局 logger。重复调用会覆盖旧实例（用于配置热更新）。
func Initialize(opts Options) error {
	var level zapcore.Level
	if err := level.UnmarshalText([]byte(strings.ToLower(opts.Level))); err != nil {
		return fmt.Errorf("非法日志级别 %q: %w", opts.Level, err)
	}

	// 编码器：console 面向人眼（带颜色），json 面向日志采集系统。
	encCfg := zap.NewProductionEncoderConfig()
	encCfg.EncodeTime = zapcore.ISO8601TimeEncoder // 统一时间格式，便于检索
	encCfg.EncodeDuration = zapcore.MillisDurationEncoder
	encCfg.TimeKey = "ts"
	encCfg.MessageKey = "msg"
	encCfg.CallerKey = "caller"
	encCfg.LevelKey = "level"

	var enc zapcore.Encoder
	if opts.Format == "json" {
		encCfg.EncodeLevel = zapcore.LowercaseLevelEncoder
		enc = zapcore.NewJSONEncoder(encCfg)
	} else {
		encCfg.EncodeLevel = zapcore.CapitalColorLevelEncoder
		enc = zapcore.NewConsoleEncoder(encCfg)
	}

	cores := []zapcore.Core{
		zapcore.NewCore(enc, zapcore.Lock(zapcore.AddSync(os.Stdout)), level),
	}

	// 配置了日志文件则额外写一份（按大小滚动，避免磁盘写满）。
	if opts.File != "" {
		if dir := filepath.Dir(opts.File); dir != "" && dir != "." {
			if err := os.MkdirAll(dir, 0o750); err != nil {
				return fmt.Errorf("创建日志目录失败: %w", err)
			}
		}
		writer := &lumberjack.Logger{
			Filename:   opts.File,
			MaxSize:    100, // MB
			MaxBackups: 7,   // 保留份数
			MaxAge:     30,  // 保留天数
			Compress:   true,
		}
		cores = append(cores, zapcore.NewCore(enc, zapcore.Lock(zapcore.AddSync(writer)), level))
	}

	base := zap.New(
		zapcore.NewTee(cores...),
		zap.AddCaller(),
		zap.AddStacktrace(zapcore.ErrorLevel), // 仅 error 级别带堆栈，避免日志膨胀
	)

	global.SugaredLogger = base.Sugar()
	global.zap = base
	return nil
}

// Zap 返回原始 zap logger，用于绑定强类型字段：
//
//	logger.Zap().Info("request", zap.String("path", "/api/v1/users"))
func Zap() *zap.Logger { return global.zap }

// Sync 刷新并关闭日志缓冲区（进程退出时调用）。
func Sync() {
	if global.zap != nil {
		_ = global.zap.Sync()
	}
}

// ---- 包级快捷函数，业务代码直接调用即可 ----

// Debugf 调试日志，仅 debug 级别输出。
func Debugf(format string, args ...interface{}) { global.Debugf(format, args...) }

// Infof 常规业务日志。
func Infof(format string, args ...interface{}) { global.Infof(format, args...) }

// Warnf 告警日志（可恢复的异常情况）。
func Warnf(format string, args ...interface{}) { global.Warnf(format, args...) }

// Errorf 错误日志（带堆栈）。
func Errorf(format string, args ...interface{}) { global.Errorf(format, args...) }

// Fatalf 致命错误日志，输出后进程退出。
func Fatalf(format string, args ...interface{}) { global.Fatalf(format, args...) }
