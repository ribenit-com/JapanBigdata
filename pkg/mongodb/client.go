// Package mongodb 提供MongoDB数据库操作的封装
// 主要功能包括：
// - 连接管理：创建和维护MongoDB连接
// - 数据操作：提供常用的CRUD操作封装
// - 连接池：管理连接池以提高性能
// - 错误处理：统一的错误处理和重试机制
package mongodb

import (
	"context"
	"fmt"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/writeconcern"
)

// MongoClient MongoDB客户端管理器
// 负责维护与MongoDB的连接和操作
type MongoClient struct {
	client *mongo.Client   // MongoDB官方客户端实例，用于执行所有数据库操作
	ctx    context.Context // 上下文对象，用于控制操作的生命周期和取消
}

// Config MongoDB连接配置
// 包含建立MongoDB连接所需的所有参数
type Config struct {
	URI      string        // MongoDB连接字符串，格式如：mongodb://host:port
	Database string        // 要连接的数据库名称
	Timeout  time.Duration // 连接和操作的超时时间，超过此时间将取消操作
}

// NewMongoClient 创建新的MongoDB客户端实例
// 该函数完成以下工作：
// 1. 配置MongoDB连接选项
// 2. 建立数据库连接
// 3. 验证连接是否成功
// 4. 返回可用的客户端实例
//
// 参数:
//   - cfg: MongoDB连接配置，包含连接信息和超时设置
//
// 返回:
//   - *MongoClient: 创建的客户端实例
//   - error: 如果连接失败则返回错误
func NewMongoClient(cfg *Config) (*MongoClient, error) {
	// 配置MongoDB客户端选项
	clientOpts := options.Client().
		ApplyURI(cfg.URI).                // 设置连接URI
		SetWriteConcern(writeconcern.New( // 配置写入确认
			writeconcern.W(1),                     // 写入确认级别：至少1个节点确认
			writeconcern.J(false),                 // 不等待日志写入
			writeconcern.WTimeout(10*time.Second), // 写入超时时间：10秒
		)).
		SetMaxPoolSize(100).  // 设置连接池最大连接数
		SetMinPoolSize(10).   // 设置连接池最小连接数
		SetMaxConnecting(20). // 设置最大并发连接数
		SetRetryWrites(true)  // 启用写入重试机制

	// 创建带超时的上下文
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel() // 确保资源被释放

	// 连接到MongoDB服务器
	client, err := mongo.Connect(ctx, clientOpts)
	if err != nil {
		return nil, fmt.Errorf("MongoDB连接失败: %w", err)
	}

	// 测试连接是否成功
	if err := client.Ping(ctx, nil); err != nil {
		return nil, fmt.Errorf("MongoDB Ping失败: %w", err)
	}

	// 记录连接成功日志
	log.Printf("MongoDB连接成功: %s", cfg.URI)

	// 返回初始化好的客户端实例
	return &MongoClient{
		client: client,
		ctx:    context.Background(), // 使用新的上下文用于后续操作
	}, nil
}

// SaveProxies 批量保存代理信息到MongoDB
// 该方法实现了高效的批量写入，包含以下特性：
// - 分批处理大量数据
// - 自动重试机制
// - 性能优化选项
//
// 参数:
//   - database: 目标数据库名称
//   - collection: 目标集合名称
//   - proxies: 要保存的代理信息列表
//
// 返回:
//   - error: 如果保存失败则返回错误
func (m *MongoClient) SaveProxies(database, collection string, proxies []interface{}) error {
	// 获取指定的集合
	coll := m.client.Database(database).Collection(collection)

	// 配置写入选项，优化性能
	opts := options.InsertMany().
		SetOrdered(false).                // 使用无序写入，提高性能
		SetBypassDocumentValidation(true) // 跳过文档验证，提高性能

	// 创建30秒超时的上下文
	ctx, cancel := context.WithTimeout(m.ctx, 30*time.Second)
	defer cancel()

	// 分批处理，每批最多1000条数据
	batchSize := 1000
	for i := 0; i < len(proxies); i += batchSize {
		// 计算当前批次的结束位置
		end := i + batchSize
		if end > len(proxies) {
			end = len(proxies)
		}
		batch := proxies[i:end]

		// 添加重试机制，最多重试3次
		var result *mongo.InsertManyResult
		var err error
		for retries := 0; retries < 3; retries++ {
			result, err = coll.InsertMany(ctx, batch, opts)
			if err == nil {
				break // 写入成功，跳出重试循环
			}
			log.Printf("批量写入失败(第%d次重试): %v", retries+1, err)
			time.Sleep(time.Duration(retries+1) * time.Second) // 递增重试等待时间
		}
		if err != nil {
			return fmt.Errorf("保存到MongoDB失败: %w", err)
		}
		log.Printf("成功保存批次 %d-%d，共 %d 条记录", i, end, len(result.InsertedIDs))
	}

	return nil
}

// Close 关闭MongoDB连接
// 在程序结束时调用，确保资源被正确释放
//
// 返回:
//   - error: 如果关闭连接时发生错误则返回
func (m *MongoClient) Close() error {
	return m.client.Disconnect(m.ctx)
}

// GetProxies 从MongoDB获取指定数量的代理
// 该方法实现了高效的批量查询，包含以下特性：
// - 限制返回数量
// - 自动超时控制
// - 结果解析和过滤
//
// 参数:
//   - database: 数据库名称
//   - collection: 集合名称
//   - limit: 限制返回的代理数量
//
// 返回:
//   - []string: 代理地址列表
//   - error: 如果查询失败则返回错误
func (m *MongoClient) GetProxies(database, collection string, limit int) ([]string, error) {
	// 获取集合引用
	coll := m.client.Database(database).Collection(collection)

	// 设置查询选项，限制返回数量
	opts := options.Find().SetLimit(int64(limit))

	// 创建10秒超时的上下文
	ctx, cancel := context.WithTimeout(m.ctx, 10*time.Second)
	defer cancel()

	// 执行查询
	cursor, err := coll.Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil, fmt.Errorf("查询MongoDB失败: %w", err)
	}
	defer cursor.Close(ctx)

	// 解析查询结果
	var results []map[string]interface{}
	if err = cursor.All(ctx, &results); err != nil {
		return nil, fmt.Errorf("解析查询结果失败: %w", err)
	}

	// 提取代理地址
	proxies := make([]string, 0, len(results))
	for _, result := range results {
		if proxy, ok := result["proxy"].(string); ok {
			proxies = append(proxies, proxy)
		}
	}

	return proxies, nil
}

// Client 获取MongoDB客户端实例
// 该方法提供对内部client字段的安全访问
//
// 返回:
//   - *mongo.Client: MongoDB官方客户端实例
func (m *MongoClient) Client() *mongo.Client {
	return m.client
}

// Context 获取上下文
// 该方法提供对内部ctx字段的安全访问
//
// 返回:
//   - context.Context: 当前使用的上下文对象
func (m *MongoClient) Context() context.Context {
	return m.ctx
}
