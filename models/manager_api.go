package models

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/imroc/req/v3"
	utls "github.com/refraction-networking/utls"
)

// NewAntiBotManager 创建新的 Vercel BotID 管理器
func NewAntiBotManager(jsURL, processURL string, refreshInterval, idleTimeout time.Duration) *AntiBotManager {
	ctx, cancel := context.WithCancel(context.Background())

	return &AntiBotManager{
		client:          req.C().ImpersonateChrome().SetTLSFingerprint(utls.HelloChrome_131),
		jsURL:           jsURL,
		processURL:      processURL,
		refreshInterval: refreshInterval,
		maxRetries:      3,
		idleTimeout:     idleTimeout,
		ctx:             ctx,
		cancel:          cancel,
		refreshActive:   false,
		wakeupChan:      make(chan struct{}, 1), // 带缓冲的通道避免阻塞
	}
}

// Start 启动管理器
func (m *AntiBotManager) Start() error {
	log.Println("🚀 启动 Vercel BotID 管理器")

	// 初始化访问时间
	m.lastAccessTime = time.Now()

	if err := m.refreshParameters(); err != nil {
		return fmt.Errorf("初始化参数失败: %w", err)
	}

	go m.autoRefreshLoop()

	log.Printf("✅ 参数管理器启动成功，刷新间隔: %v, 空闲超时: %v", m.refreshInterval, m.idleTimeout)
	return nil
}

// Stop 停止管理器
func (m *AntiBotManager) Stop() {
	log.Println("🛑 停止 Vercel BotID 管理器")
	m.cancel()
}

// GetXIsHuman 获取当前有效的 x-is-human 参数
func (m *AntiBotManager) GetXIsHuman() (string, error) {
	m.mu.RLock()

	m.stats.TotalRequests++

	// 更新最后访问时间
	m.lastAccessTime = time.Now()

	// 唤醒休眠的刷新循环
	if !m.refreshActive {
		m.mu.RUnlock()
		m.mu.Lock()
		if !m.refreshActive {
			log.Println("🔔 检测到请求,尝试唤醒刷新循环")
			select {
			case m.wakeupChan <- struct{}{}:
				log.Println("✅ 唤醒信号已发送")
			default:
				// 通道已满,说明已经有唤醒信号在等待
			}
		}
		m.mu.Unlock()
		m.mu.RLock()
	}

	// 检查参数是否过期
	if time.Since(m.lastUpdateTime) > 28*time.Second {
		m.mu.RUnlock()
		m.mu.Lock()
		if time.Since(m.lastUpdateTime) > 28*time.Second {
			log.Println("⚠️ 参数即将过期，强制刷新")
			if err := m.refreshParametersUnsafe(); err != nil {
				m.stats.FailedRequests++
				m.stats.LastError = err
				m.mu.Unlock()
				return "", fmt.Errorf("强制刷新参数失败: %w", err)
			}
		}
		m.mu.Unlock()
		m.mu.RLock()
	}

	if m.currentXIsHuman == "" {
		m.stats.FailedRequests++
		m.mu.RUnlock()
		return "", fmt.Errorf("参数未初始化")
	}

	m.stats.SuccessRequests++
	m.stats.CacheHits++
	result := m.currentXIsHuman
	m.mu.RUnlock()
	return result, nil
}

// IsHealthy 检查管理器是否健康
func (m *AntiBotManager) IsHealthy() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.currentXIsHuman != "" && time.Since(m.lastUpdateTime) < 30*time.Second
}

// GetStats 获取统计信息
func (m *AntiBotManager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	idleTime := time.Since(m.lastAccessTime)

	stats := map[string]interface{}{
		"totalRequests":     m.stats.TotalRequests,
		"successRequests":   m.stats.SuccessRequests,
		"failedRequests":    m.stats.FailedRequests,
		"cacheHits":         m.stats.CacheHits,
		"lastUpdateTime":    m.lastUpdateTime,
		"lastAccessTime":    m.lastAccessTime,
		"parameterAge":      time.Since(m.lastUpdateTime),
		"idleTime":          idleTime,
		"refreshInterval":   m.refreshInterval,
		"idleTimeout":       m.idleTimeout,
		"refreshActive":     m.refreshActive,
		"hasValidParameter": m.currentXIsHuman != "",
	}

	if m.stats.LastError != nil {
		stats["lastError"] = m.stats.LastError.Error()
	}

	return stats
}
