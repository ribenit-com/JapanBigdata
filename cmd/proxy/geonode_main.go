package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	geonode "japan_spider/spiders/proxyPool/geonode_com"
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

	// 启动爬虫
	errChan := make(chan error, 1)
	go func() {
		errChan <- spider.Run(ctx)
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
}
