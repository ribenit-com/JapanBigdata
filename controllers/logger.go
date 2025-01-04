// controllers/logger.go
package controllers

import (
	"log"
	"os"
	"sync"
)

// LoggerManager 提供一个线程安全的日志管理工具
// 支持动态设置日志文件和日志级别

type LoggerManager struct {
	logger   *log.Logger  // 标准库的日志对象
	file     *os.File     // 当前日志文件
	mux      sync.RWMutex // 保护日志操作的互斥锁
	logLevel string       // 当前日志级别 (INFO, WARN, ERROR, DEBUG)
}

// NewLoggerManager 创建并初始化一个新的 LoggerManager
func NewLoggerManager() *LoggerManager {
	return &LoggerManager{
		logger:   log.New(os.Stdout, "[INFO] ", log.LstdFlags),
		logLevel: "INFO",
	}
}

// SetLogFile 设置日志输出文件
// filepath: 日志文件路径
func (lm *LoggerManager) SetLogFile(filepath string) error {
	lm.mux.Lock()
	defer lm.mux.Unlock()

	// 如果已有日志文件，则关闭
	if lm.file != nil {
		lm.file.Close()
	}

	file, err := os.OpenFile(filepath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	lm.file = file
	lm.logger.SetOutput(file)
	return nil
}

// SetLogLevel 设置日志级别
// level: 日志级别 (INFO, WARN, ERROR, DEBUG)
func (lm *LoggerManager) SetLogLevel(level string) {
	lm.mux.Lock()
	defer lm.mux.Unlock()

	lm.logLevel = level
	lm.logger.SetPrefix("[" + level + "] ")
}

// Log 输出日志
// level: 日志级别
// message: 日志内容
func (lm *LoggerManager) Log(level, message string) {
	lm.mux.RLock()
	defer lm.mux.RUnlock()

	// 根据日志级别过滤
	if shouldLog(lm.logLevel, level) {
		lm.logger.Println(message)
	}
}

// shouldLog 判断是否需要记录当前级别的日志
// currentLevel: 当前设置的日志级别
// messageLevel: 当前日志的级别
func shouldLog(currentLevel, messageLevel string) bool {
	levels := map[string]int{
		"DEBUG": 1,
		"INFO":  2,
		"WARN":  3,
		"ERROR": 4,
	}

	return levels[messageLevel] >= levels[currentLevel]
}

// Close 关闭日志管理器
func (lm *LoggerManager) Close() {
	lm.mux.Lock()
	defer lm.mux.Unlock()

	if lm.file != nil {
		lm.file.Close()
	}
}
