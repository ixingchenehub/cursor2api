package logger

import (
	"log"
	"os"
	"strings"
)

// LogLevel 日志级别
type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
)

// Logger 全局日志管理器
type Logger struct {
	level   LogLevel
	verbose bool
}

var globalLogger *Logger

// Init 初始化日志管理器
func Init(levelStr string, verbose bool) {
	level := parseLogLevel(levelStr)
	globalLogger = &Logger{
		level:   level,
		verbose: verbose,
	}

	log.SetFlags(log.Ldate | log.Ltime)
	log.SetOutput(os.Stdout)

	Info("📋 日志管理器已初始化")
	Info("  └─ 日志级别: %s", levelStr)
	Info("  └─ 详细日志: %v", verbose)
}

// parseLogLevel 解析日志级别字符串
func parseLogLevel(levelStr string) LogLevel {
	switch strings.ToLower(levelStr) {
	case "debug":
		return DEBUG
	case "info":
		return INFO
	case "warn", "warning":
		return WARN
	case "error":
		return ERROR
	default:
		return INFO
	}
}

// Debug 输出 DEBUG 级别日志
func Debug(format string, v ...interface{}) {
	if globalLogger == nil || globalLogger.level > DEBUG {
		return
	}
	log.Printf("[DEBUG] "+format, v...)
}

// Info 输出 INFO 级别日志
func Info(format string, v ...interface{}) {
	if globalLogger == nil || globalLogger.level > INFO {
		return
	}
	if len(v) == 0 {
		log.Println(format)
	} else {
		log.Printf(format, v...)
	}
}

// Warn 输出 WARN 级别日志
func Warn(format string, v ...interface{}) {
	if globalLogger == nil || globalLogger.level > WARN {
		return
	}
	log.Printf("[WARN] "+format, v...)
}

// Error 输出 ERROR 级别日志
func Error(format string, v ...interface{}) {
	if globalLogger == nil || globalLogger.level > ERROR {
		return
	}
	log.Printf("[ERROR] "+format, v...)
}

// Verbose 输出详细日志 (仅在 VERBOSE_LOGGING=true 时输出)
func Verbose(format string, v ...interface{}) {
	if globalLogger == nil || !globalLogger.verbose {
		return
	}
	if len(v) == 0 {
		log.Println(format)
	} else {
		log.Printf(format, v...)
	}
}

// Fatal 输出 FATAL 日志并退出
func Fatal(format string, v ...interface{}) {
	log.Fatalf("[FATAL] "+format, v...)
}

// IsVerbose 返回是否启用详细日志
func IsVerbose() bool {
	if globalLogger == nil {
		return false
	}
	return globalLogger.verbose
}

// GetLevel 获取当前日志级别
func GetLevel() LogLevel {
	if globalLogger == nil {
		return INFO
	}
	return globalLogger.level
}
