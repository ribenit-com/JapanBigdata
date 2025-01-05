package main

import (
	"context"
	"japan_spider/controllers"
	"japan_spider/internal/spider"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

type AmazonProductSpider struct {
	spider.BaseSpider
	UserAgents []string
	Proxies    []string
	RateLimit  time.Duration
	mu         sync.Mutex
}

func NewAmazonProductSpider() *AmazonProductSpider {
	return &AmazonProductSpider{
		BaseSpider: spider.BaseSpider{
			StartURLs: []string{
				"https://www.amazon.co.jp/s?k=electronics",
				// 添加更多起始URL
			},
		},
		UserAgents: []string{
			"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
			// 添加更多 User Agent
		},
		RateLimit: 2 * time.Second, // 限制请求频率
	}
}

func (s *AmazonProductSpider) Process(ctx context.Context, url string) error {
	// 实现请求前的延迟
	time.Sleep(s.RateLimit)

	// 创建带超时的上下文
	reqCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// 实现具体的爬取逻辑
	product, err := s.scrapeProduct(reqCtx, url)
	if err != nil {
		return err
	}

	// 保存数据
	if err := s.saveProduct(product); err != nil {
		return err
	}

	return nil
}

type Product struct {
	Title       string    `json:"title"`
	Price       float64   `json:"price"`
	Description string    `json:"description"`
	ASIN        string    `json:"asin"`
	URL         string    `json:"url"`
	CreatedAt   time.Time `json:"created_at"`
}

func (s *AmazonProductSpider) scrapeProduct(ctx context.Context, url string) (*Product, error) {
	// TODO: 实现具体的产品页面解析逻辑
	return nil, nil
}

func (s *AmazonProductSpider) saveProduct(product *Product) error {
	// TODO: 实现数据保存逻辑（数据库、文件等）
	return nil
}

func main() {

	logger := controllers.NewLoggerManager()

	// 创建上下文和取消函数
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 设置信号处理
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 创建爬虫实例
	spider := NewAmazonProductSpider()

	// 启动一个 goroutine 监听信号
	go func() {
		<-sigChan
		logger.Log("INFO", "收到终止信号，开始优雅关闭...")
		cancel()
	}()

	// 创建工作池
	var wg sync.WaitGroup
	workerCount := 3
	urlChan := make(chan string, len(spider.StartURLs))

	// 启动工作协程
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for url := range urlChan {
				select {
				case <-ctx.Done():
					return
				default:
					if err := spider.Process(ctx, url); err != nil {
						logger.Log("ERROR", "处理URL失败: "+err.Error())
					}
				}
			}
		}()
	}

	// 分发URL到通道
	for _, url := range spider.StartURLs {
		urlChan <- url
	}
	close(urlChan)

	// 等待所有工作协程完成
	wg.Wait()
	logger.Log("INFO", "爬虫任务完成")
}
