package main

import (
	"context"
	"log"

	"./config"
	"./controllers"
	"./spiders"
)

func main() {
	// 初始化配置
	if err := config.Init(); err != nil {
		log.Fatalf("配置初始化失败: %v", err)
	}

	// 创建任务管理器
	taskManager := controllers.NewTaskManager()

	// 创建并启动爬虫
	spider := spiders.NewProductSpider()
	if err := spider.Init(); err != nil {
		log.Fatalf("爬虫初始化失败: %v", err)
	}

	// 启动爬虫任务
	taskManager.StartTask("product_spider", func(ctx context.Context) {
		for _, url := range spider.StartURLs {
			if err := spider.Process(ctx, url); err != nil {
				log.Printf("处理URL失败: %v", err)
			}
		}
	})

	// 保持程序运行
	select {}
}
