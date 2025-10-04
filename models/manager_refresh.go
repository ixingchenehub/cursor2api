package models

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"cursor2api/types"
)

// autoRefreshLoop 自动刷新循环(支持智能休眠)
func (m *AntiBotManager) autoRefreshLoop() {
	ticker := time.NewTicker(m.refreshInterval)
	defer ticker.Stop()

	m.mu.Lock()
	m.refreshActive = true
	m.mu.Unlock()

	log.Printf("♻️  自动刷新循环已启动 (空闲超时: %v)", m.idleTimeout)

	for {
		select {
		case <-m.ctx.Done():
			log.Println("📴 参数自动刷新已停止")
			return

		case <-ticker.C:
			m.mu.Lock()

			// 检查是否超过空闲时间
			idleTime := time.Since(m.lastAccessTime)
			if idleTime > m.idleTimeout {
				m.refreshActive = false
				m.mu.Unlock()

				log.Printf("😴 超过 %v 无请求,进入休眠模式", m.idleTimeout)

				// 进入休眠,等待唤醒信号
				select {
				case <-m.ctx.Done():
					log.Println("📴 参数自动刷新已停止")
					return
				case <-m.wakeupChan:
					m.mu.Lock()
					m.refreshActive = true
					m.mu.Unlock()
					log.Println("🔔 收到唤醒信号,恢复刷新循环")
				}
				continue
			}

			// 正常刷新流程
			log.Printf("🔄 开始定时刷新参数 (上次访问: %v 前)", idleTime.Round(time.Second))
			if err := m.refreshParametersUnsafe(); err != nil {
				log.Printf("❌ 定时刷新失败: %v", err)
				m.stats.LastError = err
			} else {
				log.Println("✅ 定时刷新成功")
			}
			m.mu.Unlock()
		}
	}
}

// refreshParameters 刷新参数（加锁版本）
func (m *AntiBotManager) refreshParameters() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.refreshParametersUnsafe()
}

// refreshParametersUnsafe 刷新参数（无锁版本）
func (m *AntiBotManager) refreshParametersUnsafe() error {
	var lastErr error

	for attempt := 1; attempt <= m.maxRetries; attempt++ {
		if attempt > 1 {
			log.Printf("🔁 第 %d 次重试刷新参数", attempt)
		}

		jsCode, err := m.downloadJS()
		if err != nil {
			lastErr = fmt.Errorf("下载JS失败: %w", err)
			if attempt < m.maxRetries {
				time.Sleep(time.Duration(attempt) * time.Second)
				continue
			}
			break
		}

		xIsHuman, err := m.getXIsHuman(jsCode)
		if err != nil {
			lastErr = fmt.Errorf("获取参数失败: %w", err)
			if attempt < m.maxRetries {
				time.Sleep(time.Duration(attempt) * time.Second)
				continue
			}
			break
		}

		m.jsCode = jsCode
		m.currentXIsHuman = xIsHuman
		m.lastUpdateTime = time.Now()

		log.Printf("✨ 参数刷新成功 (长度: %d)", len(xIsHuman))
		return nil
	}

	m.stats.FailedRequests.Add(1)
	return fmt.Errorf("重试 %d 次后仍然失败: %w", m.maxRetries, lastErr)
}

// downloadJS 下载 JavaScript 文件
func (m *AntiBotManager) downloadJS() (string, error) {
	resp, err := m.client.R().SetHeader("referer", "https://cursor.com/cn/learn").Get(m.jsURL)
	if err != nil {
		return "", fmt.Errorf("请求失败: %w", err)
	}

	if !resp.IsSuccessState() {
		return "", fmt.Errorf("HTTP状态码错误: %d", resp.StatusCode)
	}

	bodyContent := resp.String()
	if len(bodyContent) < 1000 {
		return "", fmt.Errorf("JS文件内容异常，大小: %d", len(bodyContent))
	}

	return bodyContent, nil
}

// getXIsHuman 从本地接口获取动态参数
func (m *AntiBotManager) getXIsHuman(jsCode string) (string, error) {
	requestData := map[string]string{
		"jsCode": jsCode,
	}

	var response types.ProcessResponseReq
	resp, err := m.client.R().
		SetHeader("Content-Type", "application/json").
		SetBodyJsonMarshal(requestData).
		SetSuccessResult(&response).
		Post(m.processURL)

	if err != nil {
		return "", fmt.Errorf("请求接口失败: %w", err)
	}

	if !resp.IsSuccessState() {
		return "", fmt.Errorf("接口返回错误状态码: %d", resp.StatusCode)
	}

	if !response.Success {
		return "", fmt.Errorf("接口返回失败状态")
	}

	xIsHuManBytes, err := json.Marshal(response.Data)
	if err != nil {
		return "", fmt.Errorf("序列化参数失败: %w", err)
	}

	return string(xIsHuManBytes), nil
}
