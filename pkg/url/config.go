package url

import "time"

// Config URL管理器配置
type Config struct {
	RedisKeyPrefix  string        // Redis键前缀
	MaxDepth        int           // 最大深度限制
	MaxPriority     int           // 最大优先级
	MetricsInterval time.Duration // 指标收集间隔
}
