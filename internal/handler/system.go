package handler

import (
	"context"
	"runtime"
	"time"

	"github.com/gin-gonic/gin"

	"go-backend/internal/database"
	_ "go-backend/internal/response"
	"go-backend/internal/version"
)

// SystemHandler 系统级接口：健康检查、存活探针、运行时信息。
type SystemHandler struct {
	startTime time.Time
}

// NewSystemHandler 构造系统处理器，记录进程启动时间。
func NewSystemHandler() *SystemHandler {
	return &SystemHandler{startTime: time.Now()}
}

func (h *SystemHandler) Health(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()

	// 数据库不可用时整体判定为不健康，触发下游摘流。
	status := "up"
	code := 200
	if err := database.Ping(ctx); err != nil {
		status = "degraded"
		code = 503
	}

	c.JSON(code, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"status":   status,
			"database": map[bool]string{true: "up", false: "down"}[status == "up"],
			"uptime":   time.Since(h.startTime).Round(time.Second).String(),
			"version":  version.Get(),
		},
	})
}

func (h *SystemHandler) Ready(c *gin.Context) {
	c.JSON(200, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"status":  "ok",
			"uptime":  time.Since(h.startTime).Round(time.Second).String(),
			"version": version.Get(),
		},
	})
}

func (h *SystemHandler) Info(c *gin.Context) {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)

	c.JSON(200, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"version":    version.Get(),
			"go_version": runtime.Version(),
			"goroutines": runtime.NumGoroutine(),
			"uptime":     time.Since(h.startTime).Round(time.Second).String(),
			"mem": gin.H{
				"alloc_mb":       round2(float64(ms.Alloc) / 1024 / 1024),
				"total_alloc_mb": round2(float64(ms.TotalAlloc) / 1024 / 1024),
				"sys_mb":         round2(float64(ms.Sys) / 1024 / 1024),
				"num_gc":         ms.NumGC,
			},
		},
	})
}

// round2 保留两位小数。
func round2(v float64) float64 {
	return float64(int64(v*100+0.5)) / 100
}
