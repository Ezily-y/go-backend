// Package version 存放构建期注入的版本信息。
//
// 通过 ldflags 在编译时注入：
//
//	go build -ldflags "-X go-backend/internal/version.Version=v1.0.0 \
//	  -X go-backend/internal/version.Commit=$(git rev-parse --short HEAD) \
//	  -X go-backend/internal/version.BuildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
package version

import "fmt"

// 构建期注入的变量，默认值用于本地 go run / go test。
var (
	Version   = "dev"
	Commit    = "none"
	BuildTime = "unknown"
)

// Get 返回格式化的版本描述。
func Get() string {
	return fmt.Sprintf("%s (commit=%s, built=%s)", Version, Commit, BuildTime)
}

// GetRaw 只返回版本号，便于前端做兼容性判断。
func GetRaw() string { return Version }
