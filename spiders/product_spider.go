package spiders

import (
	"context"
	"log"
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
	log.Printf("爬取商品数据: %s\n", url)
	// 实现商品爬取逻辑
	return nil
}
