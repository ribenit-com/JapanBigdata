// Package proxy 提供代理IP池的管理功能
// 包括代理的添加、获取、验证和评分等核心功能
package proxy

import (
	"fmt"
	"log"
	"net/url"
	"sync"
	"time"

	"japan_spider/pkg/mongodb"
	"japan_spider/pkg/redis"
)

// Proxy 定义单个代理的详细信息
// 包含代理的地址、协议、评分等属性
type Proxy struct {
	URL       string // 代理服务器的完整URL地址
	Protocol  string // 代理协议类型
	Available bool   // 代理当前是否可用
}

// ProxyPool 代理池的核心结构
// 管理代理列表并提供线程安全的操作方法
type ProxyPool struct {
	proxies    []*Proxy      // 代理列表，存储所有已添加的代理
	mu         sync.RWMutex  // 读写锁，保护并发访问代理列表
	maxRetries int           // 最大重试次数，超过此次数的代理将被标记为不可用
	checkURL   string        // 用于验证代理可用性的测试URL
	timeout    time.Duration // 代理请求超时时间
}

// Config 代理池配置选项
// 用于初始化代理池时的参数设置
type Config struct {
	BatchSize int           // 每批加载的代理数量
	Timeout   time.Duration // 操作超时时间
}

// MongoDBConfig MongoDB连接配置
type MongoDBConfig struct {
	URI        string // MongoDB连接URI，例如：mongodb://192.168.20.6:30643
	Database   string // 数据库名称
	Collection string // 集合名称
}

// RedisConfig Redis连接配置
type RedisConfig struct {
	Host     string // Redis主机地址：192.168.20.6
	Port     int    // Redis端口：32430
	Password string // Redis密码
	DB       int    // 数据库编号
}

// NewProxyPool 创建新的代理池实例
// 参数:
//   - config: 代理池配置信息
//
// 返回:
//   - *ProxyPool: 初始化好的代理池实例
func NewProxyPool(config Config) *ProxyPool {
	return &ProxyPool{
		proxies: make([]*Proxy, 0), // 初始化空的代理列表
		timeout: config.Timeout,    // 设置超时时间
	}
}

// AddProxy 向代理池添加新的代理
// 参数:
//   - proxyURL: 代理服务器URL
//   - protocol: 代理协议类型
//
// 返回:
//   - error: 如果添加失败则返回错误
func (p *ProxyPool) AddProxy(proxyURL string, protocol string) error {
	p.mu.Lock()         // 获取写锁，确保并发安全
	defer p.mu.Unlock() // 函数返回时释放锁

	// 验证代理URL格式是否正确
	_, err := url.Parse(proxyURL)
	if err != nil {
		return err
	}

	// 创建新的代理实例
	proxy := &Proxy{
		URL:       proxyURL,
		Protocol:  protocol,
		Available: true,
	}

	// 将代理添加到列表
	p.proxies = append(p.proxies, proxy)
	return nil
}

// GetProxy 获取一个可用代理
func (p *ProxyPool) GetProxy() *Proxy {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// 返回第一个可用的代理
	for _, proxy := range p.proxies {
		if proxy.Available {
			return proxy
		}
	}

	return nil
}

// RemoveProxy 从代理池中移除指定代理
// 参数:
//   - proxyURL: 要移除的代理URL
func (p *ProxyPool) RemoveProxy(proxyURL string) {
	p.mu.Lock()         // 获取写锁
	defer p.mu.Unlock() // 函数返回时释放锁

	// 查找并移除指定代理
	for i, proxy := range p.proxies {
		if proxy.URL == proxyURL {
			// 使用切片操作移除元素
			p.proxies = append(p.proxies[:i], p.proxies[i+1:]...)
			return
		}
	}
}

// LoadProxiesFromMongo 从MongoDB加载一组代理到Redis
// 参数:
//   - mongoClient: MongoDB客户端
//   - redisClient: Redis客户端
//   - batchSize: 每次加载的代理数量
//
// 返回:
//   - error: 如果加载失败则返回错误
func (p *ProxyPool) LoadProxiesFromMongo(mongoClient *mongodb.MongoClient, redisClient *redis.RedisClient, batchSize int) error {
	log.Printf("开始从MongoDB加载新的代理组(数量: %d)...", batchSize)

	// 从MongoDB获取代理
	proxies, err := mongoClient.GetProxies("proxy_pool", "proxies", batchSize)
	if err != nil {
		return fmt.Errorf("从MongoDB获取代理失败: %w", err)
	}

	if len(proxies) == 0 {
		return fmt.Errorf("MongoDB中没有可用的代理")
	}

	// 保存到Redis
	redisKey := "current_proxy_batch"
	if err := redisClient.SaveProxies(redisKey, proxies); err != nil {
		return fmt.Errorf("保存代理到Redis失败: %w", err)
	}

	log.Printf("成功加载 %d 个代理到Redis", len(proxies))
	return nil
}

// GetNextValidProxy 从Redis获取下一个可用的代理
// 参数:
//   - redisClient: Redis客户端
//   - mongoClient: MongoDB客户端
//
// 返回:
//   - *Proxy: 可用的代理，如果没有则返回nil
//   - error: 如果发生错误则返回
func (p *ProxyPool) GetNextValidProxy(redisClient *redis.RedisClient, mongoClient *mongodb.MongoClient) (*Proxy, error) {
	const redisKey = "current_proxy_batch"

	// 尝试从Redis获取代理
	proxyStr, err := redisClient.GetRandomProxy(redisKey)
	if err != nil || proxyStr == "" {
		// Redis中没有代理，尝试加载新的一批
		if err := p.LoadProxiesFromMongo(mongoClient, redisClient, 500); err != nil {
			return nil, fmt.Errorf("加载新代理失败: %w", err)
		}
		// 重新尝试获取
		proxyStr, err = redisClient.GetRandomProxy(redisKey)
		if err != nil {
			return nil, fmt.Errorf("从Redis获取代理失败: %w", err)
		}
	}

	// 创建代理实例
	proxy := &Proxy{
		URL:       proxyStr,
		Protocol:  "http", // 默认协议
		Available: true,
	}

	return proxy, nil
}

// RefreshProxyPool 刷新代理池
// 当Redis中的代理数量低于阈值时，从MongoDB加载新的代理
func (p *ProxyPool) RefreshProxyPool(redisClient *redis.RedisClient, mongoClient *mongodb.MongoClient, threshold int) error {
	const redisKey = "current_proxy_batch"

	// 获取当前Redis中的代理数量
	proxies, err := redisClient.GetProxies(redisKey)
	if err != nil {
		return fmt.Errorf("获取Redis代理数量失败: %w", err)
	}

	// 如果数量低于阈值，加载新的代理
	if len(proxies) < threshold {
		log.Printf("Redis中的代理数量(%d)低于阈值(%d)，开始加载新代理...", len(proxies), threshold)
		if err := p.LoadProxiesFromMongo(mongoClient, redisClient, 500); err != nil {
			return fmt.Errorf("加载新代理失败: %w", err)
		}
	}

	return nil
}
