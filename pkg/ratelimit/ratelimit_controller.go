package ratelimit

import (
	"context"
	"fmt"
	"sync"
	"time"

	"japan_spider/pkg/redis"
)

// RateLimitController 请求频率限制控制器
type RateLimitController struct {
	redisClient *redis.RedisClient  // Redis客户端，用于分布式限流
	config      Config              // 配置信息
	limiters    map[string]*Limiter // 域名对应的限制器
	mu          sync.RWMutex        // 读写锁
	metrics     *RateLimitMetrics   // 限流指标
}

// Limiter 单个限制器
type Limiter struct {
	rate       float64    // 每秒请求数
	burst      int        // 突发请求数
	tokens     float64    // 当前令牌数
	lastUpdate time.Time  // 上次更新时间
	mu         sync.Mutex // 互斥锁
}

// RateLimitMetrics 限流指标
type RateLimitMetrics struct {
	TotalRequests     int64                  // 总请求数
	ThrottledRequests int64                  // 被限流的请求数
	DomainStats       map[string]*DomainStat // 域名统计
	mu                sync.Mutex             // 互斥锁
}

// DomainStat 域名统计信息
type DomainStat struct {
	Requests    int64     // 请求数
	Throttled   int64     // 被限流数
	AverageRate float64   // 平均请求率
	LastUpdate  time.Time // 最后更新时间
}

// NewRateLimitController 创建新的限流控制器
func NewRateLimitController(redisClient *redis.RedisClient, config Config) *RateLimitController {
	rlc := &RateLimitController{
		redisClient: redisClient,
		config:      config,
		limiters:    make(map[string]*Limiter),
		metrics: &RateLimitMetrics{
			DomainStats: make(map[string]*DomainStat),
		},
	}

	// 启动指标收集
	go rlc.startMetricsCollector()
	// 启动自适应调节
	go rlc.startAdaptiveAdjustment()

	return rlc
}

// Allow 检查请求是否允许通过
func (rlc *RateLimitController) Allow(ctx context.Context, domain string) error {
	// 获取域名对应的限制器
	limiter := rlc.getLimiter(domain)

	// 检查分布式限流
	if err := rlc.checkDistributedLimit(ctx, domain); err != nil {
		rlc.recordThrottle(domain)
		return err
	}

	// 检查本地限流
	if !limiter.allow() {
		rlc.recordThrottle(domain)
		return fmt.Errorf("请求被限流: %s", domain)
	}

	// 记录请求
	rlc.recordRequest(domain)
	return nil
}

// SetRate 设置指定域名的请求速率
func (rlc *RateLimitController) SetRate(domain string, rate float64, burst int) {
	rlc.mu.Lock()
	defer rlc.mu.Unlock()

	limiter := &Limiter{
		rate:       rate,
		burst:      burst,
		tokens:     float64(burst),
		lastUpdate: time.Now(),
	}
	rlc.limiters[domain] = limiter
}

// getLimiter 获取或创建限制器
func (rlc *RateLimitController) getLimiter(domain string) *Limiter {
	rlc.mu.RLock()
	limiter, exists := rlc.limiters[domain]
	rlc.mu.RUnlock()

	if !exists {
		rlc.mu.Lock()
		// 双重检查
		if limiter, exists = rlc.limiters[domain]; !exists {
			limiter = &Limiter{
				rate:       rlc.config.DefaultRate,
				burst:      rlc.config.DefaultBurst,
				tokens:     float64(rlc.config.DefaultBurst),
				lastUpdate: time.Now(),
			}
			rlc.limiters[domain] = limiter
		}
		rlc.mu.Unlock()
	}

	return limiter
}

// allow 令牌桶算法实现
func (l *Limiter) allow() bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(l.lastUpdate).Seconds()
	l.tokens = min(float64(l.burst), l.tokens+elapsed*l.rate)
	l.lastUpdate = now

	if l.tokens >= 1 {
		l.tokens--
		return true
	}
	return false
}

// checkDistributedLimit 检查分布式限流
func (rlc *RateLimitController) checkDistributedLimit(ctx context.Context, domain string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		key := fmt.Sprintf("%s:%s:requests", rlc.config.RedisKeyPrefix, domain)

		// 使用Redis实现滑动窗口
		now := time.Now().Unix()
		windowStart := now - int64(rlc.config.WindowSize.Seconds())

		// 清理过期的请求记录
		err := rlc.redisClient.ZRemRangeByScore(key, 0, float64(windowStart))
		if err != nil {
			return err
		}

		// 添加当前请求记录
		err = rlc.redisClient.ZAdd(key, float64(now), fmt.Sprintf("%d", now))
		if err != nil {
			return err
		}

		// 获取当前窗口的请求数
		count, err := rlc.redisClient.ZCount(key, float64(windowStart), float64(now))
		if err != nil {
			return err
		}

		// 检查是否超过限制
		if count > rlc.config.WindowLimit {
			return fmt.Errorf("分布式限流: %s 超过窗口限制", domain)
		}
	}

	return nil
}

// startAdaptiveAdjustment 启动自适应调节
func (rlc *RateLimitController) startAdaptiveAdjustment() {
	ticker := time.NewTicker(rlc.config.AdjustInterval)
	defer ticker.Stop()

	for range ticker.C {
		rlc.mu.Lock()
		for domain, limiter := range rlc.limiters {
			stats := rlc.metrics.DomainStats[domain]
			if stats == nil {
				continue
			}

			// 根据成功率调整速率
			throttleRate := float64(stats.Throttled) / float64(stats.Requests)
			if throttleRate > rlc.config.ThrottleThreshold {
				// 降低速率
				limiter.rate = max(limiter.rate*0.8, rlc.config.MinRate)
			} else if throttleRate < rlc.config.ThrottleThreshold/2 {
				// 提高速率
				limiter.rate = min(limiter.rate*1.2, rlc.config.MaxRate)
			}
		}
		rlc.mu.Unlock()
	}
}

// recordRequest 记录请求
func (rlc *RateLimitController) recordRequest(domain string) {
	rlc.metrics.mu.Lock()
	defer rlc.metrics.mu.Unlock()

	rlc.metrics.TotalRequests++
	if stats, exists := rlc.metrics.DomainStats[domain]; exists {
		stats.Requests++
		stats.LastUpdate = time.Now()
	} else {
		rlc.metrics.DomainStats[domain] = &DomainStat{
			Requests:    1,
			LastUpdate:  time.Now(),
			AverageRate: rlc.config.DefaultRate,
		}
	}
}

// recordThrottle 记录被限流的请求
func (rlc *RateLimitController) recordThrottle(domain string) {
	rlc.metrics.mu.Lock()
	defer rlc.metrics.mu.Unlock()

	rlc.metrics.ThrottledRequests++
	if stats, exists := rlc.metrics.DomainStats[domain]; exists {
		stats.Throttled++
		stats.LastUpdate = time.Now()
	}
}

// GetMetrics 获取限流指标
func (rlc *RateLimitController) GetMetrics() *RateLimitMetrics {
	rlc.metrics.mu.Lock()
	defer rlc.metrics.mu.Unlock()
	return rlc.metrics
}

// min 返回两个float64中的较小值
func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// max 返回两个float64中的较大值
func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

// startMetricsCollector 启动指标收集器
func (rlc *RateLimitController) startMetricsCollector() {
	ticker := time.NewTicker(rlc.config.AdjustInterval)
	defer ticker.Stop()

	for range ticker.C {
		rlc.metrics.mu.Lock()
		// 更新各域名的平均请求率
		for _, stats := range rlc.metrics.DomainStats {
			elapsed := time.Since(stats.LastUpdate).Seconds()
			if elapsed > 0 {
				stats.AverageRate = float64(stats.Requests) / elapsed
			}
		}
		rlc.metrics.mu.Unlock()
	}
}
