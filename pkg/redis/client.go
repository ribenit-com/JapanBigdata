// Package redis 提供Redis连接和操作的封装
package redis

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/go-redis/redis/v8"
)

// RedisClient Redis客户端管理器
type RedisClient struct {
	client *redis.Client   // Redis客户端实例
	ctx    context.Context // 上下文，用于控制操作超时
}

// Config Redis连接配置
type Config struct {
	Host     string        // Redis服务器地址
	Port     int           // Redis服务器端口
	Password string        // Redis密码，如果有的话
	DB       int           // 要使用的数据库编号
	Timeout  time.Duration // 连接超时时间
}

// NewRedisClient 创建新的Redis客户端实例
func NewRedisClient(cfg *Config) (*RedisClient, error) {
	// 创建Redis客户端
	client := redis.NewClient(&redis.Options{
		Addr:         fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Password:     cfg.Password,
		DB:           cfg.DB,
		DialTimeout:  cfg.Timeout,
		ReadTimeout:  cfg.Timeout,
		WriteTimeout: cfg.Timeout,
	})

	// 创建上下文
	ctx := context.Background()

	// 测试连接
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("Redis连接失败: %w", err)
	}

	log.Printf("Redis连接成功: %s:%d", cfg.Host, cfg.Port)

	return &RedisClient{
		client: client,
		ctx:    ctx,
	}, nil
}

// Close 关闭Redis连接
func (r *RedisClient) Close() error {
	return r.client.Close()
}

// SaveProxies 保存代理列表到Redis（追加模式）
func (r *RedisClient) SaveProxies(key string, proxies []string) error {
	// 使用管道批量保存
	pipe := r.client.Pipeline()

	// 直接添加新的数据，不删除已有数据
	for _, proxy := range proxies {
		pipe.SAdd(r.ctx, key, proxy)
	}

	// 执行管道命令
	if _, err := pipe.Exec(r.ctx); err != nil {
		return fmt.Errorf("保存代理到Redis失败: %w", err)
	}

	log.Printf("成功追加 %d 个代理到Redis", len(proxies))
	return nil
}

// GetProxies 从Redis获取代理列表
func (r *RedisClient) GetProxies(key string) ([]string, error) {
	// 获取集合中的所有成员
	proxies, err := r.client.SMembers(r.ctx, key).Result()
	if err != nil {
		return nil, fmt.Errorf("从Redis获取代理失败: %w", err)
	}

	return proxies, nil
}

// RemoveProxy 从Redis删除指定的代理
func (r *RedisClient) RemoveProxy(key, proxy string) error {
	if err := r.client.SRem(r.ctx, key, proxy).Err(); err != nil {
		return fmt.Errorf("从Redis删除代理失败: %w", err)
	}
	return nil
}

// GetRandomProxy 随机获取一个代理
func (r *RedisClient) GetRandomProxy(key string) (string, error) {
	proxy, err := r.client.SRandMember(r.ctx, key).Result()
	if err != nil {
		return "", fmt.Errorf("从Redis随机获取代理失败: %w", err)
	}
	return proxy, nil
}

// RemoveKey 删除指定的key
func (r *RedisClient) RemoveKey(key string) error {
	if err := r.client.Del(r.ctx, key).Err(); err != nil {
		return fmt.Errorf("删除Redis key失败: %w", err)
	}
	return nil
}

// RPush 将数据添加到列表末尾
func (r *RedisClient) RPush(key string, value string) error {
	return r.client.RPush(r.ctx, key, value).Err()
}

// LPop 从列表头部弹出数据
func (r *RedisClient) LPop(key string) (string, error) {
	return r.client.LPop(r.ctx, key).Result()
}

// ZRemRangeByScore 删除有序集合中指定分数范围的成员
func (r *RedisClient) ZRemRangeByScore(key string, min, max float64) error {
	return r.client.ZRemRangeByScore(r.ctx, key, fmt.Sprintf("%f", min), fmt.Sprintf("%f", max)).Err()
}

// ZAdd 添加成员到有序集合
func (r *RedisClient) ZAdd(key string, score float64, member string) error {
	return r.client.ZAdd(r.ctx, key, &redis.Z{Score: score, Member: member}).Err()
}

// ZCount 获取有序集合中指定分数范围的成员数量
func (r *RedisClient) ZCount(key string, min, max float64) (int, error) {
	return int(r.client.ZCount(r.ctx, key, fmt.Sprintf("%f", min), fmt.Sprintf("%f", max)).Val()), nil
}

// SIsMember 检查成员是否存在于集合中
func (r *RedisClient) SIsMember(key string, member string) (bool, error) {
	return r.client.SIsMember(r.ctx, key, member).Result()
}

// SAdd 添加成员到集合
func (r *RedisClient) SAdd(key string, member string) error {
	return r.client.SAdd(r.ctx, key, member).Err()
}

// SCard 获取集合成员数量
func (r *RedisClient) SCard(key string) (int, error) {
	return int(r.client.SCard(r.ctx, key).Val()), nil
}

// SMembers 获取集合所有成员
func (r *RedisClient) SMembers(key string) ([]string, error) {
	return r.client.SMembers(r.ctx, key).Result()
}

// Get 获取键值
func (r *RedisClient) Get(key string) (string, error) {
	return r.client.Get(r.ctx, key).Result()
}

// HSet 设置哈希表字段的值
func (c *RedisClient) HSet(key, field, value string) error {
	ctx := context.Background()
	return c.client.HSet(ctx, key, field, value).Err()
}

// HGet 获取哈希表字段的值
func (c *RedisClient) HGet(key, field string) (string, error) {
	ctx := context.Background()
	return c.client.HGet(ctx, key, field).Result()
}

// Expire 设置键的过期时间
func (r *RedisClient) Expire(key string, expiration time.Duration) error {
	return r.client.Expire(r.ctx, key, expiration).Err()
}

// Ping 测试Redis连接
func (r *RedisClient) Ping() error {
	return r.client.Ping(r.ctx).Err()
}

// Exists 检查键是否存在
func (c *RedisClient) Exists(key string) (bool, error) {
	result, err := c.client.Exists(c.ctx, key).Result()
	return result > 0, err
}

// SetEX 设置键值对，带过期时间
func (c *RedisClient) SetEX(key string, value interface{}, expiration time.Duration) error {
	return c.client.SetEX(c.ctx, key, value, expiration).Err()
}

// Keys 获取匹配模式的所有键
func (c *RedisClient) Keys(pattern string) ([]string, error) {
	return c.client.Keys(c.ctx, pattern).Result()
}
