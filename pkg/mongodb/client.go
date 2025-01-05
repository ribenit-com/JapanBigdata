// Package mongodb 提供MongoDB数据库操作的封装
// 包含连接管理、数据存储等功能
package mongodb

import (
	"context"
	"fmt"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/writeconcern"
)

// MongoClient MongoDB客户端管理器
// 负责维护与MongoDB的连接和操作
type MongoClient struct {
	client *mongo.Client   // MongoDB官方客户端实例
	ctx    context.Context // 用于控制操作生命周期的上下文
}

// Config MongoDB连接配置
// 包含建立MongoDB连接所需的所有参数
type Config struct {
	URI      string        // MongoDB连接字符串，格式如：mongodb://host:port
	Database string        // 要连接的数据库名称
	Timeout  time.Duration // 连接和操作的超时时间
}

// NewMongoClient 创建新的MongoDB客户端实例
// 参数:
//   - cfg: MongoDB连接配置，包含连接信息和超时设置
//
// 返回:
//   - *MongoClient: 创建的客户端实例
//   - error: 如果连接失败则返回错误
func NewMongoClient(cfg *Config) (*MongoClient, error) {
	// 配置MongoDB客户端选项
	clientOpts := options.Client().
		ApplyURI(cfg.URI).
		SetWriteConcern(writeconcern.New(
			writeconcern.W(1),                     // 写入确认级别
			writeconcern.J(false),                 // 不等待日志写入
			writeconcern.WTimeout(10*time.Second), // 写入超时时间
		)).
		SetMaxPoolSize(100).  // 连接池大小
		SetMinPoolSize(10).   // 最小连接数
		SetMaxConnecting(20). // 最大并发连接数
		SetRetryWrites(true)  // 启用重试写入

	// 创建带超时的上下文
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	// 连接到MongoDB服务器
	client, err := mongo.Connect(ctx, clientOpts)
	if err != nil {
		return nil, fmt.Errorf("MongoDB连接失败: %w", err)
	}

	// 测试连接
	if err := client.Ping(ctx, nil); err != nil {
		return nil, fmt.Errorf("MongoDB Ping失败: %w", err)
	}

	log.Printf("MongoDB连接成功: %s", cfg.URI)

	return &MongoClient{
		client: client,
		ctx:    context.Background(),
	}, nil
}

// SaveProxies 批量保存代理信息到MongoDB
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

	// 配置写入选项
	opts := options.InsertMany().
		SetOrdered(false).                // 使用无序写入，提高性能
		SetBypassDocumentValidation(true) // 跳过文档验证，提高性能

	// 创建带超时的上下文
	ctx, cancel := context.WithTimeout(m.ctx, 30*time.Second) // 增加超时时间
	defer cancel()

	// 使用批量写入，每批最多1000条
	batchSize := 1000
	for i := 0; i < len(proxies); i += batchSize {
		end := i + batchSize
		if end > len(proxies) {
			end = len(proxies)
		}
		batch := proxies[i:end]

		// 添加重试机制
		var result *mongo.InsertManyResult
		var err error
		for retries := 0; retries < 3; retries++ {
			result, err = coll.InsertMany(ctx, batch, opts)
			if err == nil {
				break
			}
			log.Printf("批量写入失败(第%d次重试): %v", retries+1, err)
			time.Sleep(time.Duration(retries+1) * time.Second)
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
// 返回:
//   - error: 如果关闭连接时发生错误则返回
func (m *MongoClient) Close() error {
	return m.client.Disconnect(m.ctx)
}
