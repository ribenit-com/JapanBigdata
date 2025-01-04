// spiders/base_spider.go
package spiders

import (
	"context"
	"log"
	"time"
)

// BaseSpider 定义了一个通用的爬虫基类
// 包含初始化、处理单个 URL 和清理等核心功能
type BaseSpider struct {
	Name        string        // 爬虫名称，用于标识该爬虫
	Description string        // 爬虫描述，提供爬虫的详细信息
	StartURLs   []string      // 起始 URL 列表，爬虫从这些 URL 开始爬取
	Timeout     time.Duration // 每个任务的超时时间
}

// Init 初始化爬虫，例如加载配置或设置初始状态
// 返回错误信息表示初始化失败
func (s *BaseSpider) Init() error {
	log.Printf("[%s] 初始化爬虫: %s\n", time.Now().Format(time.RFC3339), s.Name)
	return nil
}

// Process 处理单个 URL 的逻辑
// ctx: 上下文对象，用于控制任务的生命周期
// url: 当前需要处理的 URL
// 返回错误信息表示处理失败
func (s *BaseSpider) Process(ctx context.Context, url string) error {
	log.Printf("[%s] 开始处理 URL: %s\n", time.Now().Format(time.RFC3339), url)

	// 模拟 URL 处理逻辑
	select {
	case <-ctx.Done():
		log.Printf("[%s] 任务被取消: %s\n", time.Now().Format(time.RFC3339), url)
		return ctx.Err()
	case <-time.After(s.Timeout):
		// 模拟处理完成
		log.Printf("[%s] 完成处理 URL: %s\n", time.Now().Format(time.RFC3339), url)
	}

	return nil
}

// Cleanup 清理爬虫，例如释放资源或保存状态
// 返回错误信息表示清理失败
func (s *BaseSpider) Cleanup() error {
	log.Printf("[%s] 清理爬虫: %s\n", time.Now().Format(time.RFC3339), s.Name)
	return nil
}

// Run 执行爬虫任务
// 遍历 StartURLs 并调用 Process 方法处理每个 URL
func (s *BaseSpider) Run() {
	log.Printf("[%s] 开始运行爬虫: %s\n", time.Now().Format(time.RFC3339), s.Name)

	ctx, cancel := context.WithTimeout(context.Background(), s.Timeout)
	defer cancel()

	for _, url := range s.StartURLs {
		if err := s.Process(ctx, url); err != nil {
			log.Printf("[%s] URL 处理失败: %s, 错误: %v\n", time.Now().Format(time.RFC3339), url, err)
		}
	}

	if err := s.Cleanup(); err != nil {
		log.Printf("[%s] 爬虫清理失败: %s, 错误: %v\n", time.Now().Format(time.RFC3339), s.Name, err)
	}
}
