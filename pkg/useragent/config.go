package useragent

import "time"

// Config UA控制器配置
type Config struct {
	Database       string        // MongoDB数据库名
	Collection     string        // MongoDB集合名
	DefaultUA      string        // 默认UA
	UpdateInterval time.Duration // 更新间隔
}
