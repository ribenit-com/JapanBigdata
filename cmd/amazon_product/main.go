package main

import (
	"japan_spider/config"
	"japan_spider/internal/spider"
	"japan_spider/pkg/crawlab"
	"japan_spider/spiders/amazon"
	"log"
	"time"
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
		BaseSpider: spider.BaseSpider{
			Name:        "amazon_product",
			Description: "Amazon product spider",
			StartURLs:   []string{"http://example.com/products"},
			Timeout:     time.Duration(config.GlobalConfig.Spider.Timeout) * time.Second,
		},
		Client: client,
	}

	if err := spider.Run(); err != nil {
		log.Fatalf("爬虫运行失败: %v", err)
	}
}
