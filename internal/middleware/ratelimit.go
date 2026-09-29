package middleware

import (
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// bucketMap 维护 clientKey → *rate.Limiter 的映射，并定期清理空闲条目，
// 防止长时间运行后内存被无界增长的 IP 列表耗尽。
type bucketMap struct {
	mu       sync.RWMutex
	limiters map[string]*bucketEntry
	r        rate.Limit
	burst    int
	lastGC   time.Time
}

// bucketEntry 包装限流器与其最后访问时间，供 LRU 式清理使用。
type bucketEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// newBucketMap 创建限流器集合。
// r 为每秒允许的请求数，burst 为令牌桶容量（瞬时可放行量）。
func newBucketMap(r rate.Limit, burst int) *bucketMap {
	return &bucketMap{
		limiters: make(map[string]*bucketEntry),
		r:        r,
		burst:    burst,
		lastGC:   time.Now(),
	}
}

// allow 判断指定 key 是否被放行。
func (m *bucketMap) allow(key string) bool {
	m.gc()

	m.mu.RLock()
	e, ok := m.limiters[key]
	m.mu.RUnlock()

	if ok {
		m.mu.Lock()
		e.lastSeen = time.Now()
		m.mu.Unlock()
		return e.limiter.Allow()
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	// 双检：并发下可能刚被别的协程创建，直接复用避免重复初始化。
	if e, ok := m.limiters[key]; ok {
		e.lastSeen = time.Now()
		return e.limiter.Allow()
	}
	limiter := rate.NewLimiter(m.r, m.burst)
	m.limiters[key] = &bucketEntry{limiter: limiter, lastSeen: time.Now()}
	return limiter.Allow()
}

// idleTTL 空闲条目保留时长。
const idleTTL = 10 * time.Minute

// gc 每隔一段时间清理长时间未访问的条目。
// 采用「懒触发」而非后台 goroutine，省掉一个需要随进程退出的定时器。
func (m *bucketMap) gc() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	if now.Sub(m.lastGC) < time.Minute {
		return
	}
	m.lastGC = now

	for k, e := range m.limiters {
		if now.Sub(e.lastSeen) > idleTTL {
			delete(m.limiters, k)
		}
	}
}

// size 返回当前维护的条目数，仅用于测试与监控。
func (m *bucketMap) size() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.limiters)
}
