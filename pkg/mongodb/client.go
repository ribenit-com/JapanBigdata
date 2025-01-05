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
	// 创建带超时的上下文
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel() // 确保资源被释放

	// 创建MongoDB客户端配置
	clientOptions := options.Client().ApplyURI(cfg.URI)

	// 连接到MongoDB服务器
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		return nil, fmt.Errorf("MongoDB连接失败: %w", err)
	}

	// 测试连接是否成功
	err = client.Ping(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("MongoDB Ping失败: %w", err)
	}

	// 记录连接成功日志
	log.Printf("MongoDB连接成功: %s", cfg.URI)

	// 返回封装后的客户端实例
	return &MongoClient{
		client: client,
		ctx:    context.Background(), // 创建新的后台上下文
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

	// 创建带10秒超时的上下文
	ctx, cancel := context.WithTimeout(m.ctx, 10*time.Second)
	defer cancel() // 确保资源被释放

	// 执行批量插入操作
	result, err := coll.InsertMany(ctx, proxies)
	if err != nil {
		return fmt.Errorf("保存到MongoDB失败: %w", err)
	}

	// 记录保存成功的数量
	log.Printf("成功保存 %d 条记录到MongoDB", len(result.InsertedIDs))
	return nil
}

// Close 关闭MongoDB连接
// 在程序结束时调用，确保资源被正确释放
// 返回:
//   - error: 如果关闭连接时发生错误则返回
func (m *MongoClient) Close() error {
	return m.client.Disconnect(m.ctx)
}
