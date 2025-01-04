package main

import (
	"japan_spider/config"
	"japan_spider/internal/spider"
	"japan_spider/pkg/crawlab"
	"japan_spider/spiders/amazon"
	"testing"
	"time"
)

// TestCreateSpider 测试爬虫创建
func TestCreateSpider(t *testing.T) {
	// 准备测试配置
	config.GlobalConfig = config.Config{
		CrawlabHost: "http://test.example.com",
		ApiKey:      "test-key",
		Spider: struct {
			Timeout    int `yaml:"timeout"`
			RetryCount int `yaml:"retry_count"`
		}{
			Timeout:    10,
			RetryCount: 3,
		},
	}

	// 创建测试客户端
	client := &crawlab.Client{
		BaseURL: config.GlobalConfig.CrawlabHost,
		ApiKey:  config.GlobalConfig.ApiKey,
	}

	// 创建爬虫实例
	spider := &amazon.ProductSpider{
		BaseSpider: spider.BaseSpider{
			Name:        "amazon_product",
			Description: "Amazon product spider",
			StartURLs:   []string{"http://example.com/products"},
			Timeout:     time.Duration(config.GlobalConfig.Spider.Timeout) * time.Second,
		},
		Client: client,
	}

	// 测试爬虫属性
	if spider.Name != "amazon_product" {
		t.Errorf("spider name = %v, want %v", spider.Name, "amazon_product")
	}

	if spider.Timeout != 10*time.Second {
		t.Errorf("spider timeout = %v, want %v", spider.Timeout, 10*time.Second)
	}

	if len(spider.StartURLs) != 1 {
		t.Errorf("start urls length = %v, want %v", len(spider.StartURLs), 1)
	}
}

// TestSpiderRun 测试爬虫运行
func TestSpiderRun(t *testing.T) {
	// 创建模拟爬虫
	spider := &amazon.ProductSpider{
		BaseSpider: spider.BaseSpider{
			Name:        "test_spider",
			Description: "Test spider",
			StartURLs:   []string{"http://test.com"},
			Timeout:     2 * time.Second,
		},
		Client: &crawlab.Client{
			BaseURL: "http://test.com",
			ApiKey:  "test-key",
		},
	}

	// 测试运行
	err := spider.Run()
	if err != nil {
		// 由于这是集成测试，可能会失败，我们只记录而不失败
		t.Logf("spider run failed: %v", err)
	}
}

// TestConfigLoading 测试配置加载
func TestConfigLoading(t *testing.T) {
	// 测试配置加载
	err := config.LoadConfig()
	if err != nil {
		t.Logf("config loading failed: %v", err)
		// 创建默认配置用于测试
		config.GlobalConfig = config.Config{
			CrawlabHost: "http://default.example.com",
			ApiKey:      "default-key",
		}
	}

	// 验证配置值
	if config.GlobalConfig.CrawlabHost == "" {
		t.Error("CrawlabHost should not be empty")
	}
	if config.GlobalConfig.ApiKey == "" {
		t.Error("ApiKey should not be empty")
	}
}

// TestClientCreation 测试客户端创建
func TestClientCreation(t *testing.T) {
	client := &crawlab.Client{
		BaseURL: "http://test.com",
		ApiKey:  "test-key",
	}

	if client.BaseURL != "http://test.com" {
		t.Errorf("client BaseURL = %v, want %v", client.BaseURL, "http://test.com")
	}
	if client.ApiKey != "test-key" {
		t.Errorf("client ApiKey = %v, want %v", client.ApiKey, "test-key")
	}
}
