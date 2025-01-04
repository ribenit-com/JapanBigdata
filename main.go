package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"japan_spider/config"
	"japan_spider/controllers"
	"japan_spider/spiders"
)

func main() {
	// 初始化配置
	if err := config.LoadConfig(); err != nil {
		log.Fatalf("配置初始化失败: %v", err)
	}
	log.Println("配置加载成功")

	// 创建并初始化日志管理器
	logger := controllers.NewLoggerManager()
	defer logger.Close()
	logger.SetLogLevel(config.GlobalConfig.Log.Level)
	logger.Log("INFO", "日志系统初始化成功")

	// 创建任务管理器
	taskManager := controllers.NewTaskManager(config.GlobalConfig.Node.MaxTasks)
	logger.Log("INFO", "任务管理器初始化成功")

	// 创建并配置爬虫实例
	spider := &spiders.ProductSpider{
		BaseSpider: spiders.BaseSpider{
			Name:        "ProductSpider",
			Description: "用于爬取商品信息的爬虫",
			StartURLs: []string{
				"http://example.com/page1",
				"http://example.com/page2",
			},
			Timeout: time.Duration(config.GlobalConfig.Spider.Timeout) * time.Second,
		},
	}

	// 初始化爬虫
	if err := spider.Init(); err != nil {
		logger.Log("ERROR", "爬虫初始化失败: "+err.Error())
		return
	}
	logger.Log("INFO", "爬虫初始化成功")

	// 设置信号处理
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 启动爬虫任务
	if err := taskManager.StartTask("product_spider", func(ctx context.Context) {
		for _, url := range spider.StartURLs {
			select {
			case <-ctx.Done():
				logger.Log("INFO", "任务被取消")
				return
			default:
				if err := spider.Process(ctx, url); err != nil {
					logger.Log("ERROR", "处理URL失败: "+err.Error())
				}
			}
		}

		if err := spider.Cleanup(); err != nil {
			logger.Log("ERROR", "清理爬虫失败: "+err.Error())
		}
	}); err != nil {
		logger.Log("ERROR", "启动任务失败: "+err.Error())
		return
	}

	logger.Log("INFO", "爬虫任务已启动，等待完成...")

	// 等待信号
	sig := <-sigChan
	logger.Log("INFO", "收到信号: "+sig.String()+", 准备退出...")
}
