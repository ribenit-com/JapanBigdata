package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
)

// ProxyInfo 代理IP信息结构
type ProxyInfo struct {
	IP         string   `json:"ip"`
	Port       string   `json:"port"`
	Protocols  []string `json:"protocols"`
	Country    string   `json:"country_name"`
	Speed      float64  `json:"speed"`
	Uptime     float64  `json:"uptime"`
	LastCheck  string   `json:"last_checked"`
	Anonymous  bool     `json:"anonymity"`
	WorkingPct float64  `json:"reliability"`
}

// GeonodeSpider 代理IP爬虫结构
type GeonodeSpider struct {
	Name        string
	Description string
	StartURLs   []string
	UserAgents  []string
	RateLimit   time.Duration
	MaxRetries  int
	Timeout     time.Duration
	mu          sync.Mutex
	results     []ProxyInfo
	client      *http.Client
	stats       *Stats
}

// Stats 爬虫统计信息
type Stats struct {
	StartTime    time.Time
	TotalURLs    int
	SuccessCount int
	ErrorCount   int
	mu           sync.Mutex
}

// APIResponse 响应结构体
type APIResponse struct {
	Status string      `json:"status"`
	Data   []ProxyInfo `json:"data"`
	Total  int         `json:"total"`
	Page   int         `json:"page"`
	Limit  int         `json:"limit"`
}

// NewGeonodeSpider 创建新的爬虫实例
func NewGeonodeSpider() *GeonodeSpider {
	return &GeonodeSpider{
		Name:        "geonode_spider",
		Description: "用于爬取代理IP的爬虫",
		StartURLs:   make([]string, 0),
		UserAgents: []string{
			"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
			"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
		},
		RateLimit:  10 * time.Second,
		MaxRetries: 3,
		Timeout:    30 * time.Second,
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

	// 使用切片收集错误，而不是通道
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
	
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		time.Sleep(s.RateLimit)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		log.Printf("创建请求失败: %v, URL: %s", err, url)
		return fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("User-Agent", s.getRandomUserAgent())
	req.Header.Set("Accept", "application/json")

	log.Printf("发送请求: %s", url)
	resp, err := s.client.Do(req)
	if err != nil {
		log.Printf("HTTP请求失败: %v, URL: %s", err, url)
		return fmt.Errorf("HTTP请求失败: %w", err)
	}
	defer resp.Body.Close()

	log.Printf("收到响应: %s, 状态码: %d", url, resp.StatusCode)
	
	// 检查状态码
	if resp.StatusCode == http.StatusTooManyRequests {
		log.Printf("请求频率过高，等待30秒后重试...")
		time.Sleep(30 * time.Second)
		return fmt.Errorf("请求频率过高")
	}
	
	if resp.StatusCode != http.StatusOK {
		log.Printf("非预期的状态码: %d, URL: %s", resp.StatusCode, url)
		return fmt.Errorf("非预期的状态码: %d", resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("读取响应失败: %v, URL: %s", err, url)
		return fmt.Errorf("读取响应失败: %w", err)
	}

	log.Printf("解析响应数据: %s, 长度: %d", url, len(body))
	
	// 打印原始响应用于调试
	log.Printf("原始响应: %s", string(body))

	var response APIResponse
	if err := json.Unmarshal(body, &response); err != nil {
		log.Printf("解析JSON失败: %v, 响应内容: %s", err, string(body))
		return fmt.Errorf("解析JSON失败: %w", err)
	}

	if response.Data == nil {
		log.Printf("响应数据为空: %s", url)
		return fmt.Errorf("响应数据为空")
	}

	s.mu.Lock()
	s.results = append(s.results, response.Data...)
	proxyCount := len(response.Data)
	totalCount := len(s.results)
	s.mu.Unlock()

	log.Printf("URL %s 成功爬取 %d 个代理，当前总数: %d", url, proxyCount, totalCount)
	return nil
}

// validateAndDeduplicateResults 验证和去重结果
func (s *GeonodeSpider) validateAndDeduplicateResults() {
	log.Printf("开始验证和去重，当前总数: %d", len(s.results))
	s.mu.Lock()
	defer s.mu.Unlock()

	seen := make(map[string]bool)
	unique := make([]ProxyInfo, 0)
	validCount := 0
	totalCount := len(s.results)

	for i, proxy := range s.results {
		if i%100 == 0 { // 每处理100个打印一次进度
			log.Printf("验证进度: %d/%d", i, totalCount)
		}

		key := fmt.Sprintf("%s:%s", proxy.IP, proxy.Port)
		if !seen[key] {
			if s.validateProxy(proxy) {
				seen[key] = true
				unique = append(unique, proxy)
				validCount++
				log.Printf("发现有效代理: %s:%s (%d/%d)", proxy.IP, proxy.Port, validCount, i+1)
			}
		}
	}

	s.results = unique
	log.Printf("验证和去重完成，原始数量: %d, 有效数量: %d", totalCount, len(s.results))
}

// validateProxy 验证代理是否可用
func (s *GeonodeSpider) validateProxy(proxy ProxyInfo) bool {
	if len(proxy.Protocols) == 0 {
		return false
	}

	proxyURL := fmt.Sprintf("%s://%s:%s",
		strings.ToLower(proxy.Protocols[0]), proxy.IP, proxy.Port)

	proxyURLParsed, err := url.Parse(proxyURL)
	if err != nil {
		return false
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			Proxy: http.ProxyURL(proxyURLParsed),
		},
	}

	resp, err := client.Get("http://httpbin.org/ip")
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

// saveResults 保存爬取结果
func (s *GeonodeSpider) saveResults() error {
	log.Printf("开始保存结果...")
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.results) == 0 {
		return fmt.Errorf("没有数据可保存")
	}

	log.Printf("创建保存目录...")
	if err := os.MkdirAll("proxies", 0755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}

	filename := fmt.Sprintf("proxies/proxies_%s.json",
		time.Now().Format("20060102_150405"))

	log.Printf("序列化数据...")
	data, err := json.MarshalIndent(s.results, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化结果失败: %w", err)
	}

	log.Printf("写入文件: %s", filename)
	if err := ioutil.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("保存文件失败: %w", err)
	}

	log.Printf("成功保存 %d 个代理到文件: %s\n", len(s.results), filename)
	return nil
}

// getRandomUserAgent 随机获取一个User-Agent
func (s *GeonodeSpider) getRandomUserAgent() string {
	if len(s.UserAgents) == 0 {
		return "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/91.0.4472.124"
	}
	return s.UserAgents[time.Now().UnixNano()%int64(len(s.UserAgents))]
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
			log.Printf("创建请求失败: %v", err)
			return 0, err
		}

		req.Header.Set("User-Agent", s.getRandomUserAgent())
		req.Header.Set("Accept", "application/json")

		resp, err := s.client.Do(req)
		if err != nil {
			log.Printf("请求失败: %v", err)
			return 0, err
		}

		body, err := ioutil.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			log.Printf("读取响应失败: %v", err)
			return 0, err
		}

		var testResp APIResponse
		if err := json.Unmarshal(body, &testResp); err != nil {
			log.Printf("解析响应失败: %v, 响应内容: %s", err, string(body))
			right = mid - 1
			continue
		}

		if len(testResp.Data) > 0 {
			lastValidPage = mid
			left = mid + 1
			log.Printf("页面 %d 有效，包含 %d 条数据", mid, len(testResp.Data))
		} else {
			right = mid - 1
			log.Printf("页面 %d 无数据", mid)
		}

		time.Sleep(5 * time.Second)
	}

	if lastValidPage == 0 {
		return 0, fmt.Errorf("未找到有效页面")
	}

	log.Printf("找到最后一个有效页面: %d", lastValidPage)
	return lastValidPage, nil
}

// 添加调试开关
var debug = os.Getenv("GEONODE_DEBUG") == "true"

func debugLog(format string, v ...interface{}) {
	if debug {
		log.Printf("[DEBUG] "+format, v...)
	}
}

func main() {
	// 设置日志格式
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds | log.Lshortfile)
	
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	spider := NewGeonodeSpider()
	log.Printf("爬虫初始化完成: %+v", spider)

	errChan := make(chan error, 1)
	go func() {
		err := spider.Run(ctx)
		log.Printf("爬虫运行结束，错误: %v", err)
		errChan <- err
	}()

	select {
	case <-sigChan:
		log.Println("收到终止信号，正在优雅关闭...")
		cancel()
		if err := <-errChan; err != nil {
			log.Printf("关闭时发生错误: %v", err)
		}
	case err := <-errChan:
		if err != nil {
			log.Printf("爬虫运行失败: %v", err)
			os.Exit(1)
		}
	}

	log.Println("爬虫已完成")
}
