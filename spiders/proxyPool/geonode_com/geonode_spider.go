// Package geonode 实现了针对 geonode.com 的代理IP爬虫
package geonode

import (
	"context"
	"encoding/json"
	"fmt"
	"japan_spider/internal/spider" // 导入基础爬虫框架
	"japan_spider/pkg/crawlab"     // 导入Crawlab客户端
	"net/http"
)

// GeonodeSpider 定义了 geonode.com 代理IP爬虫的结构
// 继承自基础爬虫，并添加了Crawlab客户端用于数据上传
type GeonodeSpider struct {
	spider.BaseSpider                 // 继承基础爬虫功能
	Client            *crawlab.Client // Crawlab客户端，用于上传采集到的代理IP数据
}

// Process 实现代理IP的爬取逻辑
// ctx: 用于控制爬虫生命周期的上下文
// url: 待处理的目标URL（geonode的API或网页URL）
// 返回可能的错误信息
func (s *GeonodeSpider) Process(ctx context.Context, url string) error {
	// 1. 创建 HTTP 请求
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// 2. 发送请求
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to fetch URL: %w", err)
	}
	defer resp.Body.Close()

	// 3. 解析响应数据（假设返回 JSON 格式）
	var proxies []string
	if err := json.NewDecoder(resp.Body).Decode(&proxies); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	return nil
}
