package models

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/imroc/req/v3"
)

// AntiBotManager Vercel BotID 参数动态管理器
type AntiBotManager struct {
	mu         sync.RWMutex
	client     *req.Client
	jsURL      string
	processURL string

	// 缓存数据
	currentXIsHuman string
	jsCode          string
	lastUpdateTime  time.Time
	lastAccessTime  time.Time // 最后一次访问时间

	// 配置参数
	refreshInterval time.Duration
	maxRetries      int
	idleTimeout     time.Duration // 空闲超时时间(超过此时间停止刷新)

	// 控制通道
	ctx    context.Context
	cancel context.CancelFunc

	// 刷新控制
	refreshActive bool          // 刷新循环是否活跃
	wakeupChan    chan struct{} // 唤醒信号

	// 统计信息
	stats ManagerStats
}

// ManagerStats 管理器统计信息
// All int64 fields use atomic operations for thread-safety
type ManagerStats struct {
	TotalRequests   atomic.Int64
	SuccessRequests atomic.Int64
	FailedRequests  atomic.Int64
	CacheHits       atomic.Int64
	LastError       error // Protected by AntiBotManager.mu
}
