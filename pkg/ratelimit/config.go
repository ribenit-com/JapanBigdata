package ratelimit

import "time"

// Config 限流控制器配置
type Config struct {
	RedisKeyPrefix    string        // Redis键前缀
	DefaultRate       float64       // 默认每秒请求数
	DefaultBurst      int           // 默认突发请求数
	WindowSize        time.Duration // 滑动窗口大小
	WindowLimit       int           // 窗口请求限制
	AdjustInterval    time.Duration // 自适应调节间隔
	ThrottleThreshold float64       // 限流阈值
	MinRate           float64       // 最小速率
	MaxRate           float64       // 最大速率
}
