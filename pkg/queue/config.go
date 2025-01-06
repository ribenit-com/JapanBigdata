package queue

import "time"

// Config 队列控制器配置
type Config struct {
	WorkerCount     int           // 工作协程数量
	MaxRetries      int           // 最大重试次数
	BatchSize       int           // 批处理大小
	FlushInterval   time.Duration // 刷新间隔
	MetricsInterval time.Duration // 指标收集间隔
	RedisKeyPrefix  string        // Redis键前缀
	MongoDatabase   string        // MongoDB数据库名
	MongoCollection string        // MongoDB集合名
}
