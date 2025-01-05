// Package geonode 实现了针对 geonode.com 的代理IP爬虫
package geonode

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// GeonodeSpider 代理IP爬虫结构，包含爬虫所需的所有配置和状态
type GeonodeSpider struct {
	Name        string        // 爬虫名称，用于标识和日志输出
	Description string        // 爬虫描述，说明爬虫的用途
	StartURLs   []string      // 起始URL列表，存储所有需要爬取的页面URL
	UserAgents  []string      // User-Agent列表，用于随机切换请求头
	RateLimit   time.Duration // 请求间隔时间，控制爬取速率
	MaxRetries  int           // 最大重试次数，处理临时性错误
	Timeout     time.Duration // 请求超时时间
	mu          sync.Mutex    // 互斥锁，保护并发访问
	results     []ProxyInfo   // 存储爬取到的代理信息
	client      *http.Client  // HTTP客户端，用于发送请求
	stats       *Stats        // 统计信息，记录爬虫运行状态
}

// ProxyInfo 存储单个代理IP的详细信息
type ProxyInfo struct {
	IP         string   `json:"ip"`           // 代理IP地址
	Port       string   `json:"port"`         // 代理端口
	Protocols  []string `json:"protocols"`    // 支持的协议（如HTTP、HTTPS）
	Country    string   `json:"country_name"` // 代理所在国家
	Speed      float64  `json:"speed"`        // 代理速度
	Uptime     float64  `json:"uptime"`       // 在线时间
	LastCheck  string   `json:"last_checked"` // 最后检查时间
	Anonymous  bool     `json:"anonymity"`    // 是否匿名
	WorkingPct float64  `json:"reliability"`  // 可用性百分比
}

// Stats 记录爬虫运行的统计信息
type Stats struct {
	StartTime    time.Time  // 爬虫启动时间
	TotalURLs    int        // 总URL数量
	SuccessCount int        // 成功处理的URL数量
	ErrorCount   int        // 处理失败的URL数量
	mu           sync.Mutex // 保护并发访问的互斥锁
}

// APIResponse 定义API响应的数据结构
type APIResponse struct {
	Status string      `json:"status"` // API响应状态
	Data   []ProxyInfo `json:"data"`   // 代理数据列表
	Total  int         `json:"total"`  // 总记录数
	Page   int         `json:"page"`   // 当前页码
	Limit  int         `json:"limit"`  // 每页记录数
}

// NewGeonodeSpider 创建并初始化一个新的爬虫实例
func NewGeonodeSpider() *GeonodeSpider {
	return &GeonodeSpider{
		Name:        "geonode_spider",
		Description: "用于爬取代理IP的爬虫",
		StartURLs:   make([]string, 0),
		UserAgents: []string{
			"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
			"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
		},
		RateLimit:  10 * time.Second, // 请求间隔10秒
		MaxRetries: 3,                // 最多重试3次
		Timeout:    30 * time.Second, // 请求超时30秒
		results:    make([]ProxyInfo, 0),
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		stats: &Stats{
			StartTime: time.Now(),
		},
	}
}

// Run 运行爬虫
func (s *GeonodeSpider) Run(ctx context.Context) error {
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds | log.Lshortfile)

	log.Printf("开始运行爬虫: %+v", s)
	log.Printf("启动爬虫: %s\n", s.Name)
	log.Printf("描述: %s\n", s.Description)

	// 获取总页数
	totalPages, err := s.getTotalPages(ctx)
	if err != nil {
		log.Printf("获取总页数失败: %v", err)
		return fmt.Errorf("获取总页数失败: %w", err)
	}

	// 生成所有页面的URL
	baseURL := "https://proxylist.geonode.com/api/proxy-list?limit=500&sort_by=lastChecked&sort_type=desc&page="
	for i := 1; i <= totalPages; i++ {
		s.StartURLs = append(s.StartURLs, fmt.Sprintf("%s%d", baseURL, i))
	}

	s.stats.TotalURLs = len(s.StartURLs)
	log.Printf("准备爬取 %d 个页面...\n", s.stats.TotalURLs)

	var errors []string

	// 按顺序处理URL
	for i, url := range s.StartURLs {
		select {
		case <-ctx.Done():
			log.Printf("收到取消信号，停止处理")
			return ctx.Err()
		default:
			log.Printf("处理第 %d/%d 个URL: %s", i+1, len(s.StartURLs), url)

			if err := s.processURLWithRetry(ctx, url); err != nil {
				errMsg := fmt.Sprintf("处理URL %s 失败: %v", url, err)
				log.Printf("错误: %s", errMsg)
				errors = append(errors, errMsg)
				s.stats.incrementErrorCount()
			} else {
				s.stats.incrementSuccessCount()
				log.Printf("完成第 %d/%d 个URL: %s", i+1, len(s.StartURLs), url)
			}

			log.Printf("等待10秒后处理下一个URL...")
			time.Sleep(10 * time.Second)
		}
	}

	log.Printf("URL处理阶段完成，开始后续处理...")

	// 验证和去重结果
	log.Printf("开始验证和去重结果...")
	s.validateAndDeduplicateResults()
	log.Printf("验证和去重完成")

	// 保存结果
	log.Printf("开始保存结果...")
	if err := s.saveResults(); err != nil {
		errMsg := fmt.Sprintf("保存结果失败: %v", err)
		log.Printf("错误: %s", errMsg)
		errors = append(errors, errMsg)
	}
	log.Printf("结果保存完成")

	// 打印统计信息
	log.Printf("打印统计信息...")
	s.printStats()
	log.Printf("统计信息打印完成")

	if len(errors) > 0 {
		log.Printf("爬虫运行完成，但有 %d 个错误", len(errors))
		return fmt.Errorf("爬取过程中发生以下错误:\n%s", strings.Join(errors, "\n"))
	}

	log.Printf("爬虫运行完成，无错误")
	return nil
}

// Stats 相关方法
func (s *Stats) incrementSuccessCount() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.SuccessCount++
}

func (s *Stats) incrementErrorCount() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ErrorCount++
}

// getTotalPages 获取总页数
func (s *GeonodeSpider) getTotalPages(ctx context.Context) (int, error) {
	log.Printf("开始获取总页数...")
	maxTestPage := 20
	var lastValidPage int

	// 二分查找最后一个有效页面
	left, right := 1, maxTestPage
	for left <= right {
		mid := (left + right) / 2
		log.Printf("尝试页数: %d (left=%d, right=%d)", mid, left, right)
		testURL := fmt.Sprintf("https://proxylist.geonode.com/api/proxy-list?limit=500&page=%d&sort_by=lastChecked&sort_type=desc", mid)

		req, err := http.NewRequestWithContext(ctx, "GET", testURL, nil)
		if err != nil {
			return 0, err
		}

		req.Header.Set("User-Agent", s.getRandomUserAgent())
		req.Header.Set("Accept", "application/json")

		resp, err := s.client.Do(req)
		if err != nil {
			return 0, err
		}
		defer resp.Body.Close()

		var testResp APIResponse
		if err := json.NewDecoder(resp.Body).Decode(&testResp); err != nil {
			right = mid - 1
			continue
		}

		if len(testResp.Data) > 0 {
			lastValidPage = mid
			left = mid + 1
		} else {
			right = mid - 1
		}

		time.Sleep(5 * time.Second)
	}

	if lastValidPage == 0 {
		return 0, fmt.Errorf("未找到有效页面")
	}

	log.Printf("找到最后一个有效页面: %d", lastValidPage)
	return lastValidPage, nil
}

// processURLWithRetry 处理单个URL（带重试）
func (s *GeonodeSpider) processURLWithRetry(ctx context.Context, url string) error {
	var lastErr error
	for retry := 0; retry < s.MaxRetries; retry++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if retry > 0 {
				retryDelay := time.Duration(retry) * 5 * time.Second
				log.Printf("等待 %v 后进行第 %d 次重试...", retryDelay, retry+1)
				time.Sleep(retryDelay)
			}

			err := s.scrapeURL(ctx, url)
			if err == nil {
				return nil
			}

			lastErr = err
			log.Printf("第 %d 次尝试失败: %v", retry+1, err)
		}
	}
	return fmt.Errorf("达到最大重试次数 (%d)，最后一次错误: %v", s.MaxRetries, lastErr)
}

// scrapeURL 爬取单个URL
func (s *GeonodeSpider) scrapeURL(ctx context.Context, url string) error {
	log.Printf("开始爬取URL: %s", url)
	time.Sleep(s.RateLimit)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("User-Agent", s.getRandomUserAgent())
	req.Header.Set("Accept", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP请求失败: %w", err)
	}
	defer resp.Body.Close()

	var response APIResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return fmt.Errorf("解析JSON失败: %w", err)
	}

	s.mu.Lock()
	s.results = append(s.results, response.Data...)
	s.mu.Unlock()

	return nil
}

// validateAndDeduplicateResults 验证和去重结果
func (s *GeonodeSpider) validateAndDeduplicateResults() {
	log.Printf("开始验证和去重，当前总数: %d", len(s.results))

	seen := make(map[string]bool)
	unique := make([]ProxyInfo, 0)

	for _, proxy := range s.results {
		key := fmt.Sprintf("%s:%s", proxy.IP, proxy.Port)
		if !seen[key] {
			seen[key] = true
			unique = append(unique, proxy)
		}
	}

	s.results = unique
	log.Printf("去重完成，剩余: %d", len(s.results))
}

// saveResults 保存结果
func (s *GeonodeSpider) saveResults() error {
	if len(s.results) == 0 {
		return fmt.Errorf("没有数据可保存")
	}

	data, err := json.MarshalIndent(s.results, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化结果失败: %w", err)
	}

	filename := fmt.Sprintf("proxies_%s.json", time.Now().Format("20060102_150405"))
	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("保存文件失败: %w", err)
	}

	log.Printf("成功保存 %d 个代理到文件: %s", len(s.results), filename)
	return nil
}

// getRandomUserAgent 随机获取一个User-Agent
func (s *GeonodeSpider) getRandomUserAgent() string {
	if len(s.UserAgents) == 0 {
		return "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/91.0.4472.124"
	}
	return s.UserAgents[time.Now().UnixNano()%int64(len(s.UserAgents))]
}

// printStats 打印统计信息
func (s *GeonodeSpider) printStats() {
	duration := time.Since(s.stats.StartTime)
	log.Printf("\n爬取统计:\n")
	log.Printf("- 总URL数: %d\n", s.stats.TotalURLs)
	log.Printf("- 成功数: %d\n", s.stats.SuccessCount)
	log.Printf("- 错误数: %d\n", s.stats.ErrorCount)
	log.Printf("- 有效代理数: %d\n", len(s.results))
	log.Printf("- 总耗时: %v\n", duration)
}
