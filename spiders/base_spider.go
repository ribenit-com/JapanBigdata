package spiders

import (
	"context"
	"log"
)

type BaseSpider struct {
	Name        string   // 爬虫名称，用于标识该爬虫
	Description string   // 爬虫描述，提供爬虫的详细信息
	StartURLs   []string // 起始 URL 列表，爬虫从这些 URL 开始爬取
}

// Init 初始化爬虫，例如加载配置或设置初始状态
func (s *BaseSpider) Init() error {
	log.Printf("初始化爬虫: %s\n", s.Name)
	return nil
}

// Process 处理单个 URL 的逻辑
// ctx: 上下文对象，用于控制任务的生命周期
// url: 当前需要处理的 URL
func (s *BaseSpider) Process(ctx context.Context, url string) error {
	log.Printf("处理URL: %s\n", url)
	return nil
}

// Cleanup 清理爬虫，例如释放资源或保存状态
func (s *BaseSpider) Cleanup() error {
	log.Printf("清理爬虫: %s\n", s.Name)
	return nil
}
