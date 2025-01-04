package spiders

import (
	"context"
	"log"
	"time"
)

type ProductSpider struct {
	BaseSpider
}

func NewProductSpider() *ProductSpider {
	return &ProductSpider{
		BaseSpider: BaseSpider{
			Name:        "product_spider",
			Description: "商品数据爬虫",
			StartURLs:   []string{"http://example.com/products"},
		},
	}
}

func (s *ProductSpider) Process(ctx context.Context, url string) error {
	log.Printf("开始处理URL: %s", url)

	// 模拟爬取过程
	select {
	case <-ctx.Done():
		log.Printf("任务被取消，URL: %s", url)
		return ctx.Err()
	case <-time.After(2 * time.Second):
		log.Printf("成功爬取URL: %s", url)
	}

	return nil
}
