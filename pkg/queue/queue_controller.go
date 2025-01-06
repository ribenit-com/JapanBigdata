package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"japan_spider/pkg/mongodb"
	"japan_spider/pkg/redis"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// QueueItem 队列项结构
type QueueItem struct {
	ID        string      `json:"id" bson:"_id"`          // 唯一标识
	Data      interface{} `json:"data" bson:"data"`       // 数据内容
	Status    string      `json:"status" bson:"status"`   // 处理状态：pending/processing/completed/failed
	Retries   int         `json:"retries" bson:"retries"` // 重试次数
	CreatedAt time.Time   `json:"created_at" bson:"created_at"`
	UpdatedAt time.Time   `json:"updated_at" bson:"updated_at"`
	Error     string      `json:"error,omitempty" bson:"error,omitempty"` // 错误信息
}

// QueueController 队列控制器
type QueueController struct {
	redisClient *redis.RedisClient   // Redis客户端，用于临时存储和缓冲
	mongoClient *mongodb.MongoClient // MongoDB客户端，用于持久化存储
	config      Config               // 队列配置
	handlers    map[string]Handler   // 数据处理器映射
	mu          sync.RWMutex         // 读写锁
	workerCount int                  // 工作协程数量
	ctx         context.Context      // 上下文
	cancel      context.CancelFunc   // 取消函数
	metrics     *QueueMetrics        // 队列监控指标
}

// Handler 数据处理器接口
type Handler interface {
	Process(item *QueueItem) error
}

// QueueMetrics 队列监控指标
type QueueMetrics struct {
	TotalItems     int64         // 总项目数
	ProcessedItems int64         // 已处理项目数
	FailedItems    int64         // 失败项目数
	AverageTime    time.Duration // 平均处理时间
	mu             sync.Mutex    // 指标更新锁
}

// NewQueueController 创建新的队列控制器
func NewQueueController(redisClient *redis.RedisClient, mongoClient *mongodb.MongoClient, config Config) *QueueController {
	ctx, cancel := context.WithCancel(context.Background())

	qc := &QueueController{
		redisClient: redisClient,
		mongoClient: mongoClient,
		config:      config,
		handlers:    make(map[string]Handler),
		workerCount: config.WorkerCount,
		ctx:         ctx,
		cancel:      cancel,
		metrics:     &QueueMetrics{},
	}

	// 启动工作协程
	qc.startWorkers()
	// 启动监控
	go qc.startMetricsCollector()

	return qc
}

// Push 将数据推入队列
func (qc *QueueController) Push(data interface{}) error {
	item := &QueueItem{
		ID:        generateID(), // 生成唯一ID
		Data:      data,
		Status:    "pending",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// 保存到Redis缓冲区
	if err := qc.saveToRedis(item); err != nil {
		return fmt.Errorf("保存到Redis失败: %w", err)
	}

	// 异步持久化到MongoDB
	go qc.persistToMongo(item)

	return nil
}

// RegisterHandler 注册数据处理器
func (qc *QueueController) RegisterHandler(dataType string, handler Handler) {
	qc.mu.Lock()
	defer qc.mu.Unlock()
	qc.handlers[dataType] = handler
}

// startWorkers 启动工作协程
func (qc *QueueController) startWorkers() {
	for i := 0; i < qc.workerCount; i++ {
		go qc.worker()
	}
}

// worker 工作协程
func (qc *QueueController) worker() {
	for {
		select {
		case <-qc.ctx.Done():
			return
		default:
			// 从Redis获取待处理项
			item, err := qc.getNextItem()
			if err != nil {
				time.Sleep(time.Second)
				continue
			}

			// 处理数据
			start := time.Now()
			if err := qc.processItem(item); err != nil {
				qc.handleFailure(item, err)
			} else {
				qc.handleSuccess(item)
			}
			qc.updateMetrics(time.Since(start))
		}
	}
}

// processItem 处理队列项
func (qc *QueueController) processItem(item *QueueItem) error {
	// 更新状态为处理中
	item.Status = "processing"
	item.UpdatedAt = time.Now()
	if err := qc.updateItem(item); err != nil {
		return err
	}

	// 获取对应的处理器
	handler, ok := qc.getHandler(item)
	if !ok {
		return fmt.Errorf("未找到处理器")
	}

	// 处理数据
	return handler.Process(item)
}

// handleFailure 处理失败情况
func (qc *QueueController) handleFailure(item *QueueItem, err error) {
	item.Status = "failed"
	item.Error = err.Error()
	item.Retries++
	item.UpdatedAt = time.Now()

	// 检查是否需要重试
	if item.Retries < qc.config.MaxRetries {
		// 重新入队，等待重试
		item.Status = "pending"
		qc.Push(item.Data)
	} else {
		// 持久化失败记录
		qc.persistFailure(item)
	}

	qc.metrics.mu.Lock()
	qc.metrics.FailedItems++
	qc.metrics.mu.Unlock()
}

// handleSuccess 处理成功情况
func (qc *QueueController) handleSuccess(item *QueueItem) {
	item.Status = "completed"
	item.UpdatedAt = time.Now()

	// 持久化成功记录
	qc.persistSuccess(item)

	qc.metrics.mu.Lock()
	qc.metrics.ProcessedItems++
	qc.metrics.mu.Unlock()
}

// startMetricsCollector 启动指标收集器
func (qc *QueueController) startMetricsCollector() {
	ticker := time.NewTicker(qc.config.MetricsInterval)
	defer ticker.Stop()

	for {
		select {
		case <-qc.ctx.Done():
			return
		case <-ticker.C:
			qc.collectMetrics()
		}
	}
}

// GetMetrics 获取当前队列指标
func (qc *QueueController) GetMetrics() *QueueMetrics {
	qc.metrics.mu.Lock()
	defer qc.metrics.mu.Unlock()
	return qc.metrics
}

// Close 关闭队列控制器
func (qc *QueueController) Close() {
	qc.cancel()
	// 等待所有工作协程完成
	time.Sleep(time.Second)
	// 保存最终指标
	qc.persistMetrics()
}

func generateID() string {
	return uuid.New().String()
}

// saveToRedis 保存队列项到Redis
func (qc *QueueController) saveToRedis(item *QueueItem) error {
	key := qc.config.RedisKeyPrefix + "pending"
	data, err := json.Marshal(item)
	if err != nil {
		return err
	}
	return qc.redisClient.RPush(key, string(data))
}

// persistToMongo 持久化队列项到MongoDB
func (qc *QueueController) persistToMongo(item *QueueItem) error {
	collection := qc.mongoClient.Client().Database(qc.config.MongoDatabase).Collection(qc.config.MongoCollection)
	_, err := collection.UpdateOne(
		qc.mongoClient.Context(),
		bson.M{"_id": item.ID},
		bson.M{"$set": item},
		options.Update().SetUpsert(true),
	)
	return err
}

// getNextItem 从Redis获取下一个待处理项
func (qc *QueueController) getNextItem() (*QueueItem, error) {
	key := qc.config.RedisKeyPrefix + "pending"
	data, err := qc.redisClient.LPop(key)
	if err != nil {
		return nil, err
	}

	var item QueueItem
	if err := json.Unmarshal([]byte(data), &item); err != nil {
		return nil, err
	}

	return &item, nil
}

// updateMetrics 更新队列指标
func (qc *QueueController) updateMetrics(duration time.Duration) {
	qc.metrics.mu.Lock()
	defer qc.metrics.mu.Unlock()

	qc.metrics.TotalItems++
	// 更新平均处理时间
	if qc.metrics.ProcessedItems > 0 {
		qc.metrics.AverageTime = time.Duration(
			(qc.metrics.AverageTime.Nanoseconds()*int64(qc.metrics.ProcessedItems-1) +
				duration.Nanoseconds()) / int64(qc.metrics.ProcessedItems))
	} else {
		qc.metrics.AverageTime = duration
	}
}

// updateItem 更新队列项状态
func (qc *QueueController) updateItem(item *QueueItem) error {
	// 更新Redis中的状态
	key := qc.config.RedisKeyPrefix + item.Status
	data, err := json.Marshal(item)
	if err != nil {
		return err
	}
	if err := qc.redisClient.RPush(key, string(data)); err != nil {
		return err
	}

	// 异步更新MongoDB
	go qc.persistToMongo(item)
	return nil
}

// getHandler 获取数据类型对应的处理器
func (qc *QueueController) getHandler(item *QueueItem) (Handler, bool) {
	qc.mu.RLock()
	defer qc.mu.RUnlock()

	// 根据数据类型获取对应的处理器
	if data, ok := item.Data.(map[string]interface{}); ok {
		if dataType, ok := data["type"].(string); ok {
			handler, exists := qc.handlers[dataType]
			return handler, exists
		}
	}
	return nil, false
}

// persistFailure 持久化失败记录
func (qc *QueueController) persistFailure(item *QueueItem) error {
	collection := qc.mongoClient.Client().Database(qc.config.MongoDatabase).Collection(qc.config.MongoCollection + "_failed")
	_, err := collection.InsertOne(qc.mongoClient.Context(), item)
	return err
}

// persistSuccess 持久化成功记录
func (qc *QueueController) persistSuccess(item *QueueItem) error {
	collection := qc.mongoClient.Client().Database(qc.config.MongoDatabase).Collection(qc.config.MongoCollection + "_completed")
	_, err := collection.InsertOne(qc.mongoClient.Context(), item)
	return err
}

// collectMetrics 收集当前队列指标
func (qc *QueueController) collectMetrics() {
	logStats := struct {
		TotalItems     int64
		ProcessedItems int64
		FailedItems    int64
		AverageTime    time.Duration
	}{
		TotalItems:     qc.metrics.TotalItems,
		ProcessedItems: qc.metrics.ProcessedItems,
		FailedItems:    qc.metrics.FailedItems,
		AverageTime:    qc.metrics.AverageTime,
	}
	log.Printf("Queue Stats: %+v", logStats)
	qc.persistMetrics()
}

// persistMetrics 持久化队列指标到MongoDB
func (qc *QueueController) persistMetrics() error {
	collection := qc.mongoClient.Client().Database(qc.config.MongoDatabase).Collection(qc.config.MongoCollection + "_metrics")
	_, err := collection.InsertOne(qc.mongoClient.Context(), qc.metrics)
	return err
}
