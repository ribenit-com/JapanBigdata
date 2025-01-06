package url

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sync"
	"time"

	"japan_spider/pkg/redis"
)

// URLController URL管理器
type URLController struct {
	redisClient *redis.RedisClient // Redis客户端，用于存储URL
	config      Config             // 配置信息
	filters     []Filter           // URL过滤规则
	metrics     *URLMetrics        // URL统计指标
	mu          sync.RWMutex       // 读写锁
}

// URLItem URL项
type URLItem struct {
	URL       string    `json:"url"`        // URL地址
	Depth     int       `json:"depth"`      // 当前深度
	Priority  int       `json:"priority"`   // 优先级
	Status    string    `json:"status"`     // 状态：pending/processing/completed/failed
	CreatedAt time.Time `json:"created_at"` // 创建时间
	UpdatedAt time.Time `json:"updated_at"` // 更新时间
}

// Filter URL过滤规则接口
type Filter interface {
	Allow(item *URLItem) bool
}

// URLMetrics URL统计指标
type URLMetrics struct {
	TotalURLs     int64            // 总URL数
	ProcessedURLs int64            // 已处理URL数
	FailedURLs    int64            // 失败URL数
	DepthStats    map[int]int64    // 各深度URL统计
	DomainStats   map[string]int64 // 各域名URL统计
	mu            sync.Mutex       // 互斥锁
}

// NewURLController 创建新的URL管理器
func NewURLController(redisClient *redis.RedisClient, config Config) *URLController {
	uc := &URLController{
		redisClient: redisClient,
		config:      config,
		filters:     make([]Filter, 0),
		metrics: &URLMetrics{
			DepthStats:  make(map[int]int64),
			DomainStats: make(map[string]int64),
		},
	}

	// 启动指标收集
	go uc.startMetricsCollector()

	return uc
}

// AddURL 添加新的URL
func (uc *URLController) AddURL(ctx context.Context, rawURL string, depth int, priority int) error {
	// URL规范化
	normalizedURL, err := uc.normalizeURL(rawURL)
	if err != nil {
		return err
	}

	// 检查深度限制
	if depth > uc.config.MaxDepth {
		return fmt.Errorf("超出最大深度限制: %d", uc.config.MaxDepth)
	}

	// 创建URL项
	item := &URLItem{
		URL:       normalizedURL,
		Depth:     depth,
		Priority:  priority,
		Status:    "pending",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// 应用过滤规则
	for _, filter := range uc.filters {
		if !filter.Allow(item) {
			return fmt.Errorf("URL被过滤: %s", normalizedURL)
		}
	}

	// 检查URL是否已存在
	exists, err := uc.exists(ctx, normalizedURL)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("URL已存在: %s", normalizedURL)
	}

	// 保存到Redis
	return uc.saveURL(ctx, item)
}

// GetNextURL 获取下一个待处理的URL
func (uc *URLController) GetNextURL(ctx context.Context) (*URLItem, error) {
	// 按优先级从高到低获取URL
	for priority := uc.config.MaxPriority; priority >= 0; priority-- {
		item, err := uc.getURLByPriority(ctx, priority)
		if err == nil && item != nil {
			return item, nil
		}
	}
	return nil, fmt.Errorf("没有待处理的URL")
}

// AddFilter 添加URL过滤规则
func (uc *URLController) AddFilter(filter Filter) {
	uc.mu.Lock()
	defer uc.mu.Unlock()
	uc.filters = append(uc.filters, filter)
}

// UpdateStatus 更新URL状态
func (uc *URLController) UpdateStatus(ctx context.Context, url string, status string) error {
	item, err := uc.getURLItem(ctx, url)
	if err != nil {
		return err
	}

	item.Status = status
	item.UpdatedAt = time.Now()
	return uc.saveURL(ctx, item)
}

// normalizeURL 规范化URL
func (uc *URLController) normalizeURL(rawURL string) (string, error) {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}

	// 移除片段
	parsedURL.Fragment = ""

	// 规范化路径
	if parsedURL.Path == "" {
		parsedURL.Path = "/"
	}

	return parsedURL.String(), nil
}

// exists 检查URL是否已存在
func (uc *URLController) exists(ctx context.Context, url string) (bool, error) {
	select {
	case <-ctx.Done():
		return false, ctx.Err()
	default:
		key := fmt.Sprintf("%s:urls", uc.config.RedisKeyPrefix)
		return uc.redisClient.SIsMember(key, url)
	}
}

// saveURL 保存URL到Redis
func (uc *URLController) saveURL(ctx context.Context, item *URLItem) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		urlKey := fmt.Sprintf("%s:urls", uc.config.RedisKeyPrefix)
		// 保存到URL集合
		if err := uc.redisClient.SAdd(urlKey, item.URL); err != nil {
			return err
		}

		// 保存到优先级队列
		priorityKey := fmt.Sprintf("%s:priority:%d", uc.config.RedisKeyPrefix, item.Priority)
		return uc.redisClient.RPush(priorityKey, item.URL)
	}
}

// getURLByPriority 获取指定优先级的URL
func (uc *URLController) getURLByPriority(ctx context.Context, priority int) (*URLItem, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
		key := fmt.Sprintf("%s:priority:%d", uc.config.RedisKeyPrefix, priority)
		url, err := uc.redisClient.LPop(key)
		if err != nil {
			return nil, err
		}
		return &URLItem{
			URL:      url,
			Priority: priority,
			Status:   "pending",
		}, nil
	}
}

// getURLItem 获取URL项信息
func (uc *URLController) getURLItem(ctx context.Context, url string) (*URLItem, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
		key := fmt.Sprintf("%s:url:%s", uc.config.RedisKeyPrefix, url)
		data, err := uc.redisClient.Get(key)
		if err != nil {
			return nil, err
		}
		var item URLItem
		if err := json.Unmarshal([]byte(data), &item); err != nil {
			return nil, err
		}
		return &item, nil
	}
}

// startMetricsCollector 启动指标收集器
func (uc *URLController) startMetricsCollector() {
	ticker := time.NewTicker(uc.config.MetricsInterval)
	defer ticker.Stop()

	for range ticker.C {
		uc.updateMetrics()
	}
}

// updateMetrics 更新URL统计指标
func (uc *URLController) updateMetrics() {
	uc.metrics.mu.Lock()
	defer uc.metrics.mu.Unlock()

	// 更新深度统计
	for depth := 0; depth <= uc.config.MaxDepth; depth++ {
		key := fmt.Sprintf("%s:depth:%d", uc.config.RedisKeyPrefix, depth)
		count, _ := uc.redisClient.SCard(key)
		uc.metrics.DepthStats[depth] = int64(count)
	}

	// 更新域名统计
	domainKey := fmt.Sprintf("%s:domains", uc.config.RedisKeyPrefix)
	domains, _ := uc.redisClient.SMembers(domainKey)
	for _, domain := range domains {
		key := fmt.Sprintf("%s:domain:%s", uc.config.RedisKeyPrefix, domain)
		count, _ := uc.redisClient.SCard(key)
		uc.metrics.DomainStats[domain] = int64(count)
	}
}

// GetMetrics 获取URL统计指标
func (uc *URLController) GetMetrics() *URLMetrics {
	uc.metrics.mu.Lock()
	defer uc.metrics.mu.Unlock()
	return uc.metrics
}
