package useragent

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"sync"
	"time"

	"japan_spider/pkg/mongodb"

	"go.mongodb.org/mongo-driver/bson"
)

// UserAgent 定义单个UA的结构
type UserAgent struct {
	Value      string    `json:"value" bson:"value"`             // UA字符串
	Type       string    `json:"type" bson:"type"`               // 设备类型：desktop/mobile/tablet
	Browser    string    `json:"browser" bson:"browser"`         // 浏览器类型：chrome/firefox/safari等
	OS         string    `json:"os" bson:"os"`                   // 操作系统：windows/macos/android等
	Version    string    `json:"version" bson:"version"`         // 版本号
	UpdateTime time.Time `json:"update_time" bson:"update_time"` // 更新时间
	Weight     int       `json:"weight" bson:"weight"`           // 使用权重
}

// UserAgentController UA管理器
type UserAgentController struct {
	mongoClient *mongodb.MongoClient    // MongoDB客户端
	config      Config                  // 配置信息
	uaCache     map[string][]*UserAgent // UA缓存，按设备类型分类
	customRules []Rule                  // 自定义规则列表
	mu          sync.RWMutex            // 读写锁
	lastUpdate  time.Time               // 最后更新时间
}

// Rule 自定义UA规则
type Rule struct {
	Pattern     string   `json:"pattern" bson:"pattern"`         // 匹配模式
	Types       []string `json:"types" bson:"types"`             // 适用设备类型
	Replacement string   `json:"replacement" bson:"replacement"` // 替换模板
	Weight      int      `json:"weight" bson:"weight"`           // 规则权重
}

// NewUserAgentController 创建新的UA控制器
func NewUserAgentController(mongoClient *mongodb.MongoClient, config Config) *UserAgentController {
	uac := &UserAgentController{
		mongoClient: mongoClient,
		config:      config,
		uaCache:     make(map[string][]*UserAgent),
		customRules: make([]Rule, 0),
	}

	// 初始加载UA
	if err := uac.loadUserAgents(); err != nil {
		log.Printf("初始加载UA失败: %v", err)
	}

	// 启动定期更新
	go uac.startUpdateLoop()

	return uac
}

// GetRandomUA 获取随机UA
func (uac *UserAgentController) GetRandomUA(deviceType string) string {
	uac.mu.RLock()
	defer uac.mu.RUnlock()

	// 获取指定设备类型的UA列表
	uas, ok := uac.uaCache[deviceType]
	if !ok || len(uas) == 0 {
		// 如果没有指定类型的UA，返回默认UA
		return uac.config.DefaultUA
	}

	// 根据权重随机选择
	totalWeight := 0
	for _, ua := range uas {
		totalWeight += ua.Weight
	}

	if totalWeight == 0 {
		// 如果总权重为0，直接随机选择
		return uas[rand.Intn(len(uas))].Value
	}

	// 按权重随机选择
	r := rand.Intn(totalWeight)
	for _, ua := range uas {
		r -= ua.Weight
		if r < 0 {
			return ua.Value
		}
	}

	return uac.config.DefaultUA
}

// AddCustomRule 添加自定义规则
func (uac *UserAgentController) AddCustomRule(rule Rule) error {
	uac.mu.Lock()
	defer uac.mu.Unlock()

	// 验证规则
	if rule.Pattern == "" {
		return fmt.Errorf("规则模式不能为空")
	}

	// 添加规则
	uac.customRules = append(uac.customRules, rule)

	// 保存到MongoDB
	return uac.saveCustomRules()
}

// UpdateUADatabase 更新UA数据库
func (uac *UserAgentController) UpdateUADatabase(uas []*UserAgent) error {
	uac.mu.Lock()
	defer uac.mu.Unlock()

	// 更新MongoDB
	collection := uac.mongoClient.Client().Database(uac.config.Database).Collection(uac.config.Collection)

	// 清空现有数据
	if err := collection.Drop(uac.mongoClient.Context()); err != nil {
		return fmt.Errorf("清空UA集合失败: %w", err)
	}

	// 批量插入新数据
	docs := make([]interface{}, len(uas))
	for i, ua := range uas {
		docs[i] = ua
	}

	_, err := collection.InsertMany(uac.mongoClient.Context(), docs)
	if err != nil {
		return fmt.Errorf("批量插入UA失败: %w", err)
	}

	// 更新缓存
	uac.updateCache(uas)
	uac.lastUpdate = time.Now()

	return nil
}

// loadUserAgents 从MongoDB加载UA
func (uac *UserAgentController) loadUserAgents() error {
	collection := uac.mongoClient.Client().Database(uac.config.Database).Collection(uac.config.Collection)

	cursor, err := collection.Find(uac.mongoClient.Context(), bson.M{})
	if err != nil {
		return fmt.Errorf("查询UA失败: %w", err)
	}
	defer cursor.Close(uac.mongoClient.Context())

	var uas []*UserAgent
	if err := cursor.All(uac.mongoClient.Context(), &uas); err != nil {
		return fmt.Errorf("解析UA数据失败: %w", err)
	}

	uac.updateCache(uas)
	return nil
}

// updateCache 更新UA缓存
func (uac *UserAgentController) updateCache(uas []*UserAgent) {
	newCache := make(map[string][]*UserAgent)

	// 按设备类型分类
	for _, ua := range uas {
		newCache[ua.Type] = append(newCache[ua.Type], ua)
	}

	// 更新缓存
	uac.uaCache = newCache
}

// startUpdateLoop 启动定期更新循环
func (uac *UserAgentController) startUpdateLoop() {
	ticker := time.NewTicker(uac.config.UpdateInterval)
	defer ticker.Stop()

	for range ticker.C {
		if err := uac.loadUserAgents(); err != nil {
			log.Printf("更新UA失败: %v", err)
		}
	}
}

// saveCustomRules 保存自定义规则到MongoDB
func (uac *UserAgentController) saveCustomRules() error {
	collection := uac.mongoClient.Client().Database(uac.config.Database).Collection(uac.config.Collection + "_rules")

	// 清空现有规则
	if err := collection.Drop(uac.mongoClient.Context()); err != nil {
		return fmt.Errorf("清空规则集合失败: %w", err)
	}

	// 保存新规则
	docs := make([]interface{}, len(uac.customRules))
	for i, rule := range uac.customRules {
		docs[i] = rule
	}

	_, err := collection.InsertMany(uac.mongoClient.Context(), docs)
	if err != nil {
		return fmt.Errorf("保存规则失败: %w", err)
	}

	return nil
}

// GetUAsByType 获取指定类型的所有UA
func (uac *UserAgentController) GetUAsByType(deviceType string) []*UserAgent {
	uac.mu.RLock()
	defer uac.mu.RUnlock()

	if uas, ok := uac.uaCache[deviceType]; ok {
		return uas
	}
	return nil
}

// ExportUAs 导出UA数据
func (uac *UserAgentController) ExportUAs() (string, error) {
	uac.mu.RLock()
	defer uac.mu.RUnlock()

	data := make(map[string][]*UserAgent)
	for typ, uas := range uac.uaCache {
		data[typ] = uas
	}

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return "", fmt.Errorf("序列化UA数据失败: %w", err)
	}

	return string(jsonData), nil
}
