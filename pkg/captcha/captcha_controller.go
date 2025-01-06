package captcha

import (
	"context"
	"encoding/base64"
	"fmt"
	"sync"
	"time"

	"japan_spider/pkg/mongodb"
)

// CaptchaController 验证码处理控制器
type CaptchaController struct {
	mongoClient *mongodb.MongoClient
	config      Config
	solvers     map[string]Solver
	cache       sync.Map // 缓存已解决的验证码
	metrics     *Metrics
}

// Solver 验证码解决器接口
type Solver interface {
	Solve(ctx context.Context, data []byte) (string, error)
	Train(samples []Sample) error
}

// Sample 训练样本
type Sample struct {
	Image    []byte
	Solution string
	Type     string
}

// Metrics 验证码统计
type Metrics struct {
	Total   int64
	Success int64
	Failed  int64
	AvgTime time.Duration
	mu      sync.Mutex
}

// NewCaptchaController 创建验证码控制器
func NewCaptchaController(client *mongodb.MongoClient, cfg Config) *CaptchaController {
	cc := &CaptchaController{
		mongoClient: client,
		config:      cfg,
		solvers:     make(map[string]Solver),
		metrics:     &Metrics{},
	}
	go cc.cleanCache()
	return cc
}

// RegisterSolver 注册验证码解决器
func (cc *CaptchaController) RegisterSolver(typ string, solver Solver) {
	cc.solvers[typ] = solver
}

// Solve 解决验证码
func (cc *CaptchaController) Solve(ctx context.Context, typ string, data []byte) (string, error) {
	start := time.Now()
	defer func() {
		cc.updateMetrics(time.Since(start))
	}()

	// 检查缓存
	if solution, ok := cc.cache.Load(string(data)); ok {
		return solution.(string), nil
	}

	// 获取解决器
	solver, ok := cc.solvers[typ]
	if !ok {
		return "", fmt.Errorf("unsupported captcha type: %s", typ)
	}

	// 尝试自动解决
	solution, err := solver.Solve(ctx, data)
	if err == nil {
		cc.cache.Store(string(data), solution)
		return solution, nil
	}

	// 如果允许人工介入
	if cc.config.AllowManual {
		return cc.handleManual(ctx, typ, data)
	}

	return "", err
}

// handleManual 处理人工验证
func (cc *CaptchaController) handleManual(ctx context.Context, typ string, data []byte) (string, error) {
	b64Data := base64.StdEncoding.EncodeToString(data)

	// 保存到MongoDB等待人工处理
	id := fmt.Sprintf("manual_%d", time.Now().UnixNano())
	doc := map[string]interface{}{
		"_id":       id,
		"type":      typ,
		"data":      b64Data,
		"status":    "pending",
		"timestamp": time.Now(),
	}

	if _, err := cc.mongoClient.Client().Database(cc.config.Database).
		Collection("manual_captchas").InsertOne(ctx, doc); err != nil {
		return "", err
	}

	// 等待人工处理结果
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	timeout := time.After(cc.config.ManualTimeout)

	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-timeout:
			return "", fmt.Errorf("manual verification timeout")
		case <-ticker.C:
			var result struct {
				Solution string `bson:"solution"`
			}
			err := cc.mongoClient.Client().Database(cc.config.Database).
				Collection("manual_captchas").FindOne(ctx, map[string]string{"_id": id}).
				Decode(&result)
			if err == nil && result.Solution != "" {
				return result.Solution, nil
			}
		}
	}
}

// Train 训练验证码解决器
func (cc *CaptchaController) Train(typ string, samples []Sample) error {
	solver, ok := cc.solvers[typ]
	if !ok {
		return fmt.Errorf("unsupported captcha type: %s", typ)
	}
	return solver.Train(samples)
}

// GetMetrics 获取统计信息
func (cc *CaptchaController) GetMetrics() *Metrics {
	cc.metrics.mu.Lock()
	defer cc.metrics.mu.Unlock()
	return cc.metrics
}

// updateMetrics 更新统计信息
func (cc *CaptchaController) updateMetrics(duration time.Duration) {
	cc.metrics.mu.Lock()
	defer cc.metrics.mu.Unlock()
	cc.metrics.Total++
	cc.metrics.AvgTime = time.Duration(
		(int64(cc.metrics.AvgTime)*cc.metrics.Total + int64(duration)) / (cc.metrics.Total + 1))
}

// cleanCache 定期清理缓存
func (cc *CaptchaController) cleanCache() {
	ticker := time.NewTicker(cc.config.CacheCleanInterval)
	for range ticker.C {
		now := time.Now()
		cc.cache.Range(func(key, value interface{}) bool {
			if t, ok := cc.cache.Load(key.(string) + "_time"); ok {
				if now.Sub(t.(time.Time)) > cc.config.CacheTTL {
					cc.cache.Delete(key)
					cc.cache.Delete(key.(string) + "_time")
				}
			}
			return true
		})
	}
}
