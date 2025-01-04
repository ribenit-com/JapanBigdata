// controllers/logger.go
package controllers

import (
	"log"
	"os"
)

type LoggerManager struct {
	logFile *os.File
}

func NewLoggerManager() *LoggerManager {
	return &LoggerManager{}
}

func (l *LoggerManager) SetLogLevel(level string) {
	// 实现日志级别设置
}

func (l *LoggerManager) Log(level, message string) {
	log.Printf("[%s] %s", level, message)
}

func (l *LoggerManager) Close() {
	if l.logFile != nil {
		l.logFile.Close()
	}
}
