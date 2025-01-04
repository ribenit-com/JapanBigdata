package main

import (
	"context"
	"log"

	"japan_spider/config"
	"japan_spider/controllers"
	"japan_spider/spiders"
)

func main() {
	// 初始化配置
	if err := config.LoadConfig(); err != nil {
		log.Fatalf("配置初始化失败: %v", err)
	}

	// 创建日志管理器
	logger := controllers.NewLoggerManager()
	defer logger.Close() // 确保程序退出时关闭日志文件
	logger.SetLogLevel("INFO")

	// 创建任务管理器
	taskManager := controllers.NewTaskManager(1)

	// 创建并初始化爬虫
	spider := &spiders.ProductSpider{
		BaseSpider: spiders.BaseSpider{
			Name:        "ProductSpider",
			Description: "用于爬取商品信息的爬虫",
			StartURLs:   []string{"http://example.com/page1", "http://example.com/page2"},
			Timeout:     10,
		},
	}
	if err := spider.Init(); err != nil {
		logger.Log("ERROR", "爬虫初始化失败: "+err.Error())
		return
	}

	// 启动爬虫任务
	if err := taskManager.StartTask("product_spider", func(ctx context.Context) {
		for _, url := range spider.StartURLs {
			if err := spider.Process(ctx, url); err != nil {
				logger.Log("ERROR", "处理URL失败: "+err.Error())
			}
		}

		if err := spider.Cleanup(); err != nil {
			logger.Log("ERROR", "清理爬虫失败: "+err.Error())
		}
	}); err != nil {
		logger.Log("ERROR", "启动任务失败: "+err.Error())
		return
	}

	// 保持程序运行
	logger.Log("INFO", "爬虫任务已启动，等待完成...")
	select {}
}
