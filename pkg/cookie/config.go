package cookie

import "time"

// Config Cookie控制器配置
type Config struct {
	Database      string        // MongoDB数据库名
	Collection    string        // MongoDB集合名
	MaxAge        time.Duration // Cookie最大存活时间
	CheckInterval time.Duration // 过期检查间隔
}
