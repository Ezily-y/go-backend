// Package metrics 基于 Prometheus 客户端暴露应用指标。
//
// 暴露的指标：
//   - HTTP 请求量 / 耗时（按方法、路由模板、状态码分维度）
//   - 鉴权失败计数（可用于发现暴力破解）
//   - Go 运行时与进程指标（由 client_golang 默认注册，开箱即用）
package metrics

import (
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// httpRequests HTTP 请求总数。
	httpRequests = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "HTTP 请求总数，按方法、路径、状态码统计",
		},
		[]string{"method", "path", "status"},
	)

	// httpRequestDuration HTTP 请求耗时分布。
	httpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP 请求耗时分布（秒）",
			Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5},
		},
		[]string{"method", "path"},
	)

	// authFailures 鉴权失败次数，可用于发现暴力破解。
	authFailures = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "auth_failures_total",
			Help: "鉴权失败总数，按原因统计",
		},
		[]string{"reason"},
	)
)

// init 把自定义指标注册到默认注册表，这样 promhttp.Handler() 无需额外传参。
//
// 注意：Go 运行时指标（GC/goroutine/内存）与进程指标（CPU/内存/文件描述符）
// 已由 client_golang 在包初始化时自动注册到默认注册表，
// 此处重复注册会 panic，因此不再手动添加。
func init() {
	prometheus.MustRegister(httpRequests, httpRequestDuration, authFailures)
}

// Handler 返回 Prometheus 抓取端点的处理器。
func Handler() gin.HandlerFunc {
	return func(c *gin.Context) {
		promhttp.Handler().ServeHTTP(c.Writer, c.Request)
	}
}

// ObserveRequest 记录一次 HTTP 请求的量与耗时。
// path 传入的是路由模板（如 /api/v1/users/:id）而非真实路径，
// 否则每个不同 ID 都会生成一条独立的时间序列，导致指标爆炸。
func ObserveRequest(method, path string, status int, seconds float64) {
	httpRequests.WithLabelValues(method, path, itoa(status)).Inc()
	httpRequestDuration.WithLabelValues(method, path).Observe(seconds)
}

// IncAuthFailure 记录一次鉴权失败。
func IncAuthFailure(reason string) {
	authFailures.WithLabelValues(reason).Inc()
}

// itoa 是 strconv.Itoa 的轻量替代，避免为一个转换引入整包依赖。
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [4]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
