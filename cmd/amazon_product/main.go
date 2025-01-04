package main

import (
	"japan_spider/config"
	"japan_spider/pkg/crawlab"
	"japan_spider/spiders/amazon"
	"log"
)

func main() {
	// 加载配置
	if err := config.LoadConfig(); err != nil {
		log.Fatalf("配置加载失败: %v", err)
	}

	// 创建 Crawlab 客户端
	client := &crawlab.Client{
		BaseURL: config.GlobalConfig.CrawlabHost,
		ApiKey:  config.GlobalConfig.ApiKey,
	}

	// 创建并运行爬虫
	spider := &amazon.ProductSpider{
		Client: client,
	}

	if err := spider.Run(); err != nil {
		log.Fatalf("爬虫运行失败: %v", err)
	}
}
