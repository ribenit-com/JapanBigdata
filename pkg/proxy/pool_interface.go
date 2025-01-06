package proxy

import (
	"japan_spider/pkg/mongodb"
	"japan_spider/pkg/redis"
	"time"
)

// ProxyPoolInterface 定义代理池的接口
// 所有代理池实现都需要满足这个接口
type ProxyPoolInterface interface {
	// AddProxy 添加新代理到池中
	AddProxy(proxyURL string, protocol string) error

	// GetProxy 获取一个可用代理
	GetProxy() *Proxy

	// RemoveProxy 从池中移除指定代理
	RemoveProxy(proxyURL string)

	// LoadProxiesFromMongo 从MongoDB加载一组代理到Redis
	LoadProxiesFromMongo(mongoClient *mongodb.MongoClient, redisClient *redis.RedisClient, batchSize int) error

	// GetNextValidProxy 从Redis获取下一个可用代理
	GetNextValidProxy(redisClient *redis.RedisClient, mongoClient *mongodb.MongoClient) (*Proxy, error)

	// RefreshProxyPool 刷新代理池，当代理数量低于阈值时从MongoDB加载新代理
	RefreshProxyPool(redisClient *redis.RedisClient, mongoClient *mongodb.MongoClient, threshold int) error

	// GetProxyCount 获取当前池中代理数量
	GetProxyCount() int

	// GetAvailableCount 获取当前可用代理数量
	GetAvailableCount() int

	// Clear 清空代理池
	Clear()
}

// ProxyProvider 定义代理提供者的接口
// 用于不同来源的代理获取实现
type ProxyProvider interface {
	// FetchProxies 获取一批代理
	FetchProxies(batchSize int) ([]string, error)

	// ValidateProxy 验证代理是否可用
	ValidateProxy(proxyURL string) bool

	// GetSource 获取代理来源信息
	GetSource() string
}

// ProxyStorage 定义代理存储的接口
// 用于不同存储方式的实现
type ProxyStorage interface {
	// SaveProxies 保存代理列表
	SaveProxies(proxies []string) error

	// GetProxies 获取代理列表
	GetProxies() ([]string, error)

	// RemoveProxy 删除指定代理
	RemoveProxy(proxyURL string) error

	// Clear 清空存储
	Clear() error
}

// ProxyStats 定义代理统计的接口
// 用于代理使用情况统计
type ProxyStats interface {
	// IncrementSuccess 增加成功次数
	IncrementSuccess(proxyURL string)

	// IncrementFailure 增加失败次数
	IncrementFailure(proxyURL string)

	// GetStats 获取代理统计信息
	GetStats(proxyURL string) (successes, failures int)

	// ResetStats 重置统计信息
	ResetStats(proxyURL string)
}

// ProxyConfig 定义代理配置接口
// 用于管理代理池配置
type ProxyConfig interface {
	// GetBatchSize 获取批量操作大小
	GetBatchSize() int

	// GetTimeout 获取超时设置
	GetTimeout() time.Duration

	// GetRetryCount 获取重试次数
	GetRetryCount() int

	// GetRefreshThreshold 获取刷新阈值
	GetRefreshThreshold() int
}
