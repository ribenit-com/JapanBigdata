package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	geonode "japan_spider/spiders/proxyPool/geonode_com"

	"japan_spider/pkg/mongodb"
	"japan_spider/pkg/redis"
)

func main() {
	// 设置日志格式
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds | log.Lshortfile)

	// 创建上下文和取消函数
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 设置信号处理
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 创建爬虫实例
	spider := geonode.NewGeonodeSpider()
	log.Printf("爬虫初始化完成: %+v", spider)

	// 在main函数中添加Redis初始化
	redisCfg := &redis.Config{
		Host:     "192.168.20.6",
		Port:     32430,
		Password: "", // 如果没有密码就留空
		DB:       0,  // 使用默认数据库
		Timeout:  5 * time.Second,
	}

	redisClient, err := redis.NewRedisClient(redisCfg)
	if err != nil {
		log.Fatalf("Redis初始化失败: %v", err)
	}
	defer redisClient.Close()

	// 初始化MongoDB
	mongoCfg := &mongodb.Config{
		URI:      "mongodb://192.168.20.6:30643",
		Database: "proxy_pool",
		Timeout:  5 * time.Second,
	}

	mongoClient, err := mongodb.NewMongoClient(mongoCfg)
	if err != nil {
		log.Fatalf("MongoDB初始化失败: %v", err)
	}
	defer mongoClient.Close()

	// 启动爬虫
	errChan := make(chan error, 1)
	go func() {
		errChan <- spider.Run(ctx, redisClient)
	}()

	// 等待信号或完成
	select {
	case <-sigChan:
		log.Println("收到终止信号，正在优雅关闭...")
		cancel()
		if err := <-errChan; err != nil {
			log.Printf("关闭时发生错误: %v", err)
		}
	case err := <-errChan:
		if err != nil {
			log.Printf("爬虫运行失败: %v", err)
			os.Exit(1)
		}
	}

	log.Println("爬虫已完成")

	// 在爬虫完成后进行数据处理
	if err := spider.SaveToMongoDB(redisClient, mongoClient); err != nil {
		log.Printf("保存到MongoDB失败: %v", err)
	}
}
