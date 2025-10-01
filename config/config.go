package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// Config 应用配置
type Config struct {
	Server ServerConfig
	Cursor CursorConfig
	Logger LoggerConfig
	Models []ModelConfig
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Port string
}

// CursorConfig Cursor 配置
type CursorConfig struct {
	JSURL           string
	ProcessURL      string
	DefaultModel    string
	RefreshInterval time.Duration // 参数刷新间隔
	IdleTimeout     time.Duration // 空闲超时时间
	SystemPrompt    string        // 系统提示词(会自动添加到第一条 user 消息前)
}

// LoggerConfig 日志配置
type LoggerConfig struct {
	Level   string // 日志级别: debug, info, warn, error
	Verbose bool   // 是否启用详细日志
}

// ModelConfig 模型配置
type ModelConfig struct {
	ID      string
	Object  string
	OwnedBy string
}

// Load 加载配置
func Load() *Config {
	return &Config{
		Server: ServerConfig{
			Port: getEnv("PORT", "3001"),
		},
		Cursor: CursorConfig{
			JSURL:           getEnv("JS_URL", "https://cursor.com/149e9513-01fa-4fb0-aad4-566afd725d1b/2d206a39-8ed7-437e-a3be-862e0f06eea3/a-4-a/c.js?i=0&v=3&h=cursor.com"),
			ProcessURL:      getEnv("PROCESS_URL", "http://localhost:3000/api/process"),
			DefaultModel:    "anthropic/claude-opus-4.1",
			RefreshInterval: getDurationEnv("REFRESH_INTERVAL", 25*time.Second),
			IdleTimeout:     getDurationEnv("IDLE_TIMEOUT", 10*time.Minute),
			SystemPrompt:    getEnv("SYSTEM_PROMPT", "后续回答不需要读取当前站点的知识,也不需要回复我与问题无关内容"),
		},
		Logger: LoggerConfig{
			Level:   getEnv("LOG_LEVEL", "info"),
			Verbose: getBoolEnv("VERBOSE_LOGGING", false),
		},
		Models: []ModelConfig{
			{ID: "anthropic/claude-4.5-sonnet", Object: "model", OwnedBy: "cursor"},
			{ID: "anthropic/claude-4-sonnet", Object: "model", OwnedBy: "cursor"},
			{ID: "anthropic/claude-opus-4.1", Object: "model", OwnedBy: "cursor"},
			{ID: "openai/gpt-5", Object: "model", OwnedBy: "cursor"},
			{ID: "google/gemini-2.5-pro", Object: "model", OwnedBy: "cursor"},
			{ID: "xai/grok-4", Object: "model", OwnedBy: "cursor"},
		},
	}
}

// getEnv 获取环境变量
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getDurationEnv 获取时长类型的环境变量(支持秒为单位)
func getDurationEnv(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		// 尝试解析为秒数
		if seconds, err := strconv.Atoi(value); err == nil {
			return time.Duration(seconds) * time.Second
		}
		// 尝试解析为 Go duration 格式 (如 "25s", "10m", "1h")
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}

// getBoolEnv 获取布尔类型的环境变量
func getBoolEnv(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		value = strings.ToLower(strings.TrimSpace(value))
		return value == "true" || value == "1" || value == "yes" || value == "on"
	}
	return defaultValue
}
