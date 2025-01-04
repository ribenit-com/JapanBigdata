// controllers/logger.go
package controllers

import (
	"log"
	"os"
	"sync"
)

// LoggerManager 提供日志管理功能
// 支持日志级别控制和文件输出
type LoggerManager struct {
	logFile  *os.File     // 日志文件句柄
	mu       sync.RWMutex // 保护并发写入
	logLevel string       // 当前日志级别
}

// NewLoggerManager 创建新的日志管理器实例
func NewLoggerManager() *LoggerManager {
	return &LoggerManager{
		logLevel: "INFO", // 默认日志级别
	}
}

// SetLogLevel 设置日志级别
// level: 可以是 "DEBUG", "INFO", "WARN", "ERROR"
func (l *LoggerManager) SetLogLevel(level string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.logLevel = level
}

// Log 记录日志信息
// level: 日志级别
// message: 日志内容
func (l *LoggerManager) Log(level, message string) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	log.Printf("[%s] %s", level, message)
}

// Close 关闭日志管理器，清理资源
func (l *LoggerManager) Close() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.logFile != nil {
		l.logFile.Close()
	}
}
