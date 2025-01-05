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
	"japan_spider/internal/spider"
	"japan_spider/spiders/amazon"
)

func main() {
	// 初始化配置，从配置文件加载全局设置
	if err := config.LoadConfig(); err != nil {
		// 如果配置加载失败，记录错误并立即退出程序
		log.Fatalf("配置初始化失败: %v", err)
	}
	log.Println("配置加载成功")

	// 创建并初始化日志管理器，用于集中管理日志输出
	logger := controllers.NewLoggerManager()
	// 确保在程序退出时关闭日志文件
	defer logger.Close()
	// 从配置文件设置日志级别
	logger.SetLogLevel(config.GlobalConfig.Log.Level)
	logger.Log("INFO", "日志系统初始化成功")

	// 创建任务管理器，控制并发任务数量
	taskManager := controllers.NewTaskManager(config.GlobalConfig.Node.MaxTasks)
	logger.Log("INFO", "任务管理器初始化成功")

	// 创建爬虫实例并配置基本参数
	spider := &amazon.ProductSpider{
		BaseSpider: spider.BaseSpider{
			Name:        "ProductSpider",
			Description: "用于爬取商品信息的爬虫",
			// 设置起始URL列表，爬虫将从这些URL开始爬取
			StartURLs: []string{
				"http://example.com/page1",
				"http://example.com/page2",
			},
			// 从配置文件读取超时时间并转换为Duration类型
			Timeout: time.Duration(config.GlobalConfig.Spider.Timeout) * time.Second,
		},
	}

	// 初始化爬虫，执行必要的准备工作
	if err := spider.Init(); err != nil {
		logger.Log("ERROR", "爬虫初始化失败: "+err.Error())
		return
	}
	logger.Log("INFO", "爬虫初始化成功")

	// 设置信号处理，用于优雅退出
	// 创建带缓冲的信号通道，避免信号丢失
	sigChan := make(chan os.Signal, 1)
	// 监听中断信号和终止信号
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 启动爬虫任务，使用闭包函数处理具体的爬取逻辑
	if err := taskManager.StartTask("product_spider", func(ctx context.Context) {
		// 遍历所有起始URL
		for _, url := range spider.StartURLs {
			select {
			case <-ctx.Done():
				// 如果上下文被取消，立即停止处理
				logger.Log("INFO", "任务被取消")
				return
			default:
				// 处理单个URL，如果失败则记录错误但继续处理下一个
				if err := spider.Process(ctx, url); err != nil {
					logger.Log("ERROR", "处理URL失败: "+err.Error())
				}
			}
		}

		// 任务完成后执行清理工作
		if err := spider.Cleanup(); err != nil {
			logger.Log("ERROR", "清理爬虫失败: "+err.Error())
		}
	}); err != nil {
		logger.Log("ERROR", "启动任务失败: "+err.Error())
		return
	}

	logger.Log("INFO", "爬虫任务已启动，等待完成...")

	// 等待操作系统信号
	// 这里使用无缓冲接收，因为我们只关心第一个信号
	sig := <-sigChan
	logger.Log("INFO", "收到信号: "+sig.String()+", 准备退出...")
}
