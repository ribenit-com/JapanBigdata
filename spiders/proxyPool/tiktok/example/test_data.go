package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"japan_spider/pkg/mongodb"
	"japan_spider/pkg/redis"
	"japan_spider/spiders/proxyPool/tiktok/model"

	"go.mongodb.org/mongo-driver/bson"
)

// DataViewer 数据查看器
type DataViewer struct {
	mongoClient *mongodb.MongoClient
	redisClient *redis.RedisClient
}

// NewDataViewer 创建新的数据查看器
func NewDataViewer() (*DataViewer, error) {
	// 创建MongoDB客户端
	mongoClient, err := mongodb.NewMongoClient(&mongodb.Config{
		URI:      "mongodb://192.168.20.6:30643",
		Database: "spider",
		Timeout:  5 * time.Minute,
	})
	if err != nil {
		return nil, fmt.Errorf("创建MongoDB客户端失败: %w", err)
	}

	// 创建Redis客户端
	redisClient, err := redis.NewRedisClient(&redis.Config{
		Host:     "192.168.20.6",
		Port:     32430,
		Password: "",
		DB:       0,
		Timeout:  5 * time.Minute,
	})
	if err != nil {
		return nil, fmt.Errorf("创建Redis客户端失败: %w", err)
	}

	return &DataViewer{
		mongoClient: mongoClient,
		redisClient: redisClient,
	}, nil
}

// Close 关闭连接
func (d *DataViewer) Close() error {
	var errs []error
	if err := d.mongoClient.Close(); err != nil {
		errs = append(errs, fmt.Errorf("关闭MongoDB连接失败: %w", err))
	}
	if err := d.redisClient.Close(); err != nil {
		errs = append(errs, fmt.Errorf("关闭Redis连接失败: %w", err))
	}
	if len(errs) > 0 {
		return fmt.Errorf("关闭资源时发生错误: %v", errs)
	}
	return nil
}

// ViewRedisLoginInfo 查看Redis中的登录信息
func (d *DataViewer) ViewRedisLoginInfo(email string) error {
	log.Printf("开始查询Redis中 %s 的登录信息...", email)

	// 使用模式匹配查找所有相关的key
	pattern := fmt.Sprintf("tiktok:login:%s:*", email)
	keys, err := d.redisClient.Keys(pattern)
	if err != nil {
		return fmt.Errorf("查询Redis键失败: %w", err)
	}

	log.Printf("找到 %d 条相关记录", len(keys))

	// 遍历所有key
	for _, key := range keys {
		data, err := d.redisClient.Get(key)
		if err != nil {
			log.Printf("获取key=%s的数据失败: %v", key, err)
			continue
		}

		var userInfo model.UserInfo
		if err := json.Unmarshal([]byte(data), &userInfo); err != nil {
			log.Printf("解析数据失败: %v", err)
			continue
		}

		log.Printf("\n--- Redis记录 ---")
		log.Printf("键名: %s", key)
		log.Printf("邮箱: %s", userInfo.Email)
		log.Printf("IP: %s", userInfo.IP)
		log.Printf("登录时间: %v", userInfo.LoginTime)
		log.Printf("过期时间: %v", userInfo.ExpireTime)
		log.Printf("Cookie数量: %d", len(userInfo.Cookies))
		log.Printf("登录状态: %v", userInfo.LoginStatus)
		log.Printf("Cookie有效: %v", userInfo.CookieValid)
		log.Printf("--------------\n")
	}

	return nil
}

// ViewMongoLoginInfo 查看MongoDB中的登录信息
func (d *DataViewer) ViewMongoLoginInfo(email string) error {
	log.Printf("开始查询MongoDB中 %s 的登录信息...", email)

	collection := d.mongoClient.Database("spider").Collection("tiktok_users")

	// 查找所有相关文档
	cursor, err := collection.Find(context.Background(), bson.M{"email": email})
	if err != nil {
		return fmt.Errorf("查询MongoDB失败: %w", err)
	}
	defer cursor.Close(context.Background())

	var count int
	for cursor.Next(context.Background()) {
		count++
		var userInfo model.UserInfo
		if err := cursor.Decode(&userInfo); err != nil {
			log.Printf("解析文档失败: %v", err)
			continue
		}

		log.Printf("\n--- MongoDB记录 %d ---", count)
		log.Printf("邮箱: %s", userInfo.Email)
		log.Printf("IP: %s", userInfo.IP)
		log.Printf("登录时间: %v", userInfo.LoginTime)
		log.Printf("过期时间: %v", userInfo.ExpireTime)
		log.Printf("Cookie数量: %d", len(userInfo.Cookies))
		log.Printf("登录状态: %v", userInfo.LoginStatus)
		log.Printf("Cookie有效: %v", userInfo.CookieValid)
		log.Printf("浏览器信息:")
		log.Printf("  - UserAgent: %s", userInfo.BrowserInfo.UserAgent)
		log.Printf("  - Version: %s", userInfo.BrowserInfo.Version)
		log.Printf("  - Platform: %s", userInfo.BrowserInfo.Platform)
		log.Printf("------------------\n")
	}

	if count == 0 {
		log.Printf("未找到相关记录")
	} else {
		log.Printf("共找到 %d 条记录", count)
	}

	return nil
}

// InvalidateLogin 使指定用户的登录状态失效
func (d *DataViewer) InvalidateLogin(email string) error {
	log.Printf("开始使 %s 的登录状态失效...", email)

	// 1. 使 Redis 中的登录状态失效
	pattern := fmt.Sprintf("tiktok:login:%s:*", email)
	keys, err := d.redisClient.Keys(pattern)
	if err != nil {
		return fmt.Errorf("查询Redis键失败: %w", err)
	}

	for _, key := range keys {
		data, err := d.redisClient.Get(key)
		if err != nil {
			log.Printf("获取key=%s的数据失败: %v", key, err)
			continue
		}

		var userInfo model.UserInfo
		if err := json.Unmarshal([]byte(data), &userInfo); err != nil {
			log.Printf("解析数据失败: %v", err)
			continue
		}

		// 修改状态
		userInfo.LoginStatus = false
		userInfo.CookieValid = false
		userInfo.ExpireTime = time.Now().Add(-24 * time.Hour) // 设置为过期

		// 保存回Redis
		updatedData, err := json.Marshal(userInfo)
		if err != nil {
			log.Printf("序列化数据失败: %v", err)
			continue
		}

		if err := d.redisClient.SetEX(key, string(updatedData), 7*24*time.Hour); err != nil {
			log.Printf("更新Redis数据失败: %v", err)
		}
	}

	// 2. 更新 MongoDB 中的登录状态
	collection := d.mongoClient.Database("spider").Collection("tiktok_users")
	update := bson.M{
		"$set": bson.M{
			"login_status": false,
			"cookie_valid": false,
			"expire_time":  time.Now().Add(-24 * time.Hour),
		},
	}

	_, err = collection.UpdateMany(
		context.Background(),
		bson.M{"email": email},
		update,
	)

	if err != nil {
		return fmt.Errorf("更新MongoDB数据失败: %w", err)
	}

	log.Printf("成功使 %s 的登录状态失效", email)
	return nil
}
