// Package config 负责加载、合并并校验应用配置。
//
// 加载顺序（后者覆盖前者）：
//  1. 代码内默认值
//  2. 配置文件（config/config.yaml）
//  3. 环境变量（通过 BindEnv 显式绑定的键）
//
// 设计原则：配置外置。敏感信息（JWT 密钥、数据库 DSN 等）走环境变量，
// 非敏感信息走配置文件，两者通过同一套 key 访问。
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config 是应用的根配置，对应 config.yaml 的顶层结构。
type Config struct {
	App      AppConfig      `mapstructure:"app"`
	Log      LogConfig      `mapstructure:"log"`
	Database DatabaseConfig `mapstructure:"database"`
	Redis    RedisConfig    `mapstructure:"redis"`
}

// AppConfig 应用自身的基础配置。
type AppConfig struct {
	Name        string        `mapstructure:"name"`
	Mode        string        `mapstructure:"mode"` // debug | release | test
	Port        int           `mapstructure:"port"`
	UploadDir   string        `mapstructure:"upload_dir"`
	CORSOrigins []string      `mapstructure:"cors_origins"`
	JWT         JWTConfig     `mapstructure:"jwt"`
	Bootstrap   BootstrapConf `mapstructure:"bootstrap"`
}

// JWTConfig 令牌签发配置。
type JWTConfig struct {
	Secret     string        `mapstructure:"secret"`
	AccessTTL  time.Duration `mapstructure:"access_ttl"`
	RefreshTTL time.Duration `mapstructure:"refresh_ttl"`
}

// BootstrapConf 首次启动时的种子数据（管理员账号）配置。
type BootstrapConf struct {
	Enabled  bool   `mapstructure:"enabled"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
	Role     string `mapstructure:"role"`
}

// LogConfig 日志配置。
type LogConfig struct {
	Level  string `mapstructure:"level"`  // debug | info | warn | error
	Format string `mapstructure:"format"` // console | json
	File   string `mapstructure:"file"`   // 为空表示不落盘
}

// DatabaseConfig 数据库配置。
type DatabaseConfig struct {
	Driver          string        `mapstructure:"driver"` // sqlite | postgres
	DSN             string        `mapstructure:"dsn"`
	MaxOpenConns    int           `mapstructure:"max_open_conns"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
	SlowThreshold   time.Duration `mapstructure:"slow_threshold"`
}

// RedisConfig Redis 配置（二期启用）。
type RedisConfig struct {
	Enabled  bool   `mapstructure:"enabled"`
	Addr     string `mapstructure:"addr"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

// Load 从指定路径加载配置；path 为空时使用默认的 config/config.yaml。
func Load(path string) (*Config, error) {
	v := viper.New()

	// ---- 1. 内置默认值：即使配置文件缺失也能跑起来 ----
	setDefaults(v)

	// ---- 2. 配置文件 ----
	if path == "" {
		path = defaultPath()
	}
	v.SetConfigFile(path)
	v.SetConfigType("yaml")
	if err := v.ReadInConfig(); err != nil {
		// 默认路径允许配置文件缺失（回落到默认值 + 环境变量）；
		// 但通过 CONFIG_PATH 或参数显式指定的文件读不到，属于致命错误。
		var notFound viper.ConfigFileNotFoundError
		if !errors.As(err, &notFound) && !os.IsNotExist(err) {
			return nil, fmt.Errorf("读取配置文件 %s 失败: %w", path, err)
		}
	}

	// ---- 3. 环境变量（显式绑定，键名固定可读）----
	bindEnvs(v)

	cfg := &Config{}
	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("解析配置失败: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// setDefaults 写入内置默认值，保证最小配置即可启动。
func setDefaults(v *viper.Viper) {
	v.SetDefault("app.name", "go-backend")
	v.SetDefault("app.mode", "debug")
	v.SetDefault("app.port", 8080)
	v.SetDefault("app.upload_dir", "./data/uploads")
	v.SetDefault("app.cors_origins", []string{"*"})
	v.SetDefault("app.jwt.secret", "dev-only-secret-change-me-0123456789")
	v.SetDefault("app.jwt.access_ttl", 2*time.Hour)
	v.SetDefault("app.jwt.refresh_ttl", 168*time.Hour)
	v.SetDefault("app.bootstrap.enabled", true)
	v.SetDefault("app.bootstrap.username", "admin")
	v.SetDefault("app.bootstrap.password", "admin123456")
	v.SetDefault("app.bootstrap.role", "admin")

	v.SetDefault("log.level", "info")
	v.SetDefault("log.format", "console")
	v.SetDefault("log.file", "")

	v.SetDefault("database.driver", "sqlite")
	v.SetDefault("database.dsn", "./data/app.db")
	v.SetDefault("database.max_open_conns", 20)
	v.SetDefault("database.max_idle_conns", 5)
	v.SetDefault("database.conn_max_lifetime", 30*time.Minute)
	v.SetDefault("database.slow_threshold", 200*time.Millisecond)

	v.SetDefault("redis.enabled", false)
	v.SetDefault("redis.addr", "127.0.0.1:6379")
	v.SetDefault("redis.db", 0)
}

// bindEnvs 显式绑定环境变量。
// 不使用 viper.AutomaticEnv 是因为嵌套键的自动映射规则不直观（APP_APP_JWT_SECRET 这种），
// 显式绑定换来的是可预测、可文档化的键名。
func bindEnvs(v *viper.Viper) {
	pairs := [][2]string{
		{"APP_NAME", "app.name"},
		{"APP_MODE", "app.mode"},
		{"APP_PORT", "app.port"},
		{"APP_UPLOAD_DIR", "app.upload_dir"},
		{"APP_CORS_ORIGINS", "app.cors_origins"}, // 逗号分隔
		{"JWT_SECRET", "app.jwt.secret"},
		{"JWT_ACCESS_TTL", "app.jwt.access_ttl"},
		{"JWT_REFRESH_TTL", "app.jwt.refresh_ttl"},

		{"LOG_LEVEL", "log.level"},
		{"LOG_FORMAT", "log.format"},
		{"LOG_FILE", "log.file"},

		{"DB_DRIVER", "database.driver"},
		{"DB_DSN", "database.dsn"},

		{"REDIS_ENABLED", "redis.enabled"},
		{"REDIS_ADDR", "redis.addr"},
		{"REDIS_PASSWORD", "redis.password"},
	}
	for _, p := range pairs {
		_ = v.BindEnv(p[1], p[0])
	}

	// 逗号分隔的列表型环境变量：viper 直接绑定拿到的是字符串，
	// 这里在读取后统一拆分。
	if raw := os.Getenv("APP_CORS_ORIGINS"); raw != "" {
		v.Set("app.cors_origins", strings.Split(raw, ","))
	}
	if raw := os.Getenv("APP_PORT"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil {
			v.Set("app.port", n)
		}
	}
}

// defaultPath 返回默认配置文件路径。
func defaultPath() string {
	if p := os.Getenv("CONFIG_PATH"); p != "" {
		return p
	}
	return "config/config.yaml"
}

// Validate 校验配置的合法性，在启动阶段尽早失败。
func (c *Config) Validate() error {
	if c.App.Port <= 0 || c.App.Port > 65535 {
		return fmt.Errorf("配置非法: app.port 必须在 1-65535 之间，当前 %d", c.App.Port)
	}
	switch c.App.Mode {
	case "debug", "release", "test":
	default:
		return fmt.Errorf("配置非法: app.mode 只能是 debug/release/test，当前 %q", c.App.Mode)
	}
	// JWT 使用 HS256，密钥过短会导致安全性不足，直接拒绝启动。
	if len(c.App.JWT.Secret) < 32 {
		return fmt.Errorf("配置非法: app.jwt.secret 长度必须 >= 32（可用环境变量 JWT_SECRET 覆盖）")
	}
	if c.App.JWT.AccessTTL <= 0 || c.App.JWT.RefreshTTL <= 0 {
		return fmt.Errorf("配置非法: jwt access_ttl / refresh_ttl 必须为正数")
	}
	if c.App.JWT.RefreshTTL < c.App.JWT.AccessTTL {
		return fmt.Errorf("配置非法: refresh_ttl 不应短于 access_ttl")
	}
	switch c.Database.Driver {
	case "sqlite", "postgres":
	default:
		return fmt.Errorf("配置非法: database.driver 只能是 sqlite/postgres，当前 %q", c.Database.Driver)
	}
	if strings.TrimSpace(c.Database.DSN) == "" {
		return fmt.Errorf("配置非法: database.dsn 不能为空")
	}
	switch c.Log.Level {
	case "debug", "info", "warn", "error":
	default:
		return fmt.Errorf("配置非法: log.level 只能是 debug/info/warn/error，当前 %q", c.Log.Level)
	}
	return nil
}

// Addr 返回 HTTP 监听地址，例如 ":8080"。
func (c *Config) Addr() string {
	return fmt.Sprintf(":%d", c.App.Port)
}
