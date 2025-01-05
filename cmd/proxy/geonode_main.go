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

// NewGeonodeSpider 创建新的爬虫实例
func NewGeonodeSpider() *GeonodeSpider {
	return &GeonodeSpider{
		Name:        "geonode_spider",
		Description: "用于爬取代理IP的爬虫",
		StartURLs:   []string{},
		UserAgents: []string{
			"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
			"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
			"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
		},
		RateLimit:  10 * time.Second,
		MaxRetries: 3,
		Timeout:    30 * time.Second,
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
	log.Printf("启动爬虫: %s\n", s.Name)
	log.Printf("描述: %s\n", s.Description)

	// 获取总页数
	totalPages, err := s.getTotalPages(ctx)
	if err != nil {
		return fmt.Errorf("获取总页数失败: %w", err)
	}

	// 生成所有页面的URL
	baseURL := "https://proxylist.geonode.com/api/proxy-list?limit=100&sort_by=lastChecked&sort_type=desc&protocols=http%2Chttps&page="
	for i := 1; i <= totalPages; i++ {
		s.StartURLs = append(s.StartURLs, fmt.Sprintf("%s%d", baseURL, i))
	}

	s.stats.TotalURLs = len(s.StartURLs)
	log.Printf("准备爬取 %d 个页面...\n", s.stats.TotalURLs)

	var wg sync.WaitGroup
	errChan := make(chan error, len(s.StartURLs))
	semaphore := make(chan struct{}, 3)

	for _, url := range s.StartURLs {
		wg.Add(1)
		go func(url string) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			if err := s.processURLWithRetry(ctx, url); err != nil {
				errChan <- fmt.Errorf("处理URL %s 失败: %v", url, err)
				s.stats.incrementErrorCount()
			} else {
				s.stats.incrementSuccessCount()
			}
			
			// 强制等待10秒
			time.Sleep(10 * time.Second)
		}(url)
	}

	wg.Wait()
	close(errChan)

	// 收集错误
	var errors []string
	for err := range errChan {
		errors = append(errors, err.Error())
	}

	// 验证和去重结果
	s.validateAndDeduplicateResults()

	// 保存结果
	if err := s.saveResults(); err != nil {
		errors = append(errors, fmt.Sprintf("保存结果失败: %v", err))
	}

	// 打印统计信息
	s.printStats()

	if len(errors) > 0 {
		return fmt.Errorf("爬取过程中发生以下错误:\n%s", strings.Join(errors, "\n"))
	}

	return nil
}

// processURLWithRetry 处理单个URL（带重试）
func (s *GeonodeSpider) processURLWithRetry(ctx context.Context, url string) error {
	for retry := 0; retry < s.MaxRetries; retry++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			err := s.scrapeURL(ctx, url)
			if err == nil {
				return nil
			}
			retryDelay := time.Duration(retry+1) * 2 * time.Second
			log.Printf("爬取失败 (重试 %d/%d): %v, 等待 %v 后重试\n",
				retry+1, s.MaxRetries, err, retryDelay)
			time.Sleep(retryDelay)
		}
	}
	return fmt.Errorf("达到最大重试次数")
}

// scrapeURL 爬取单个URL
func (s *GeonodeSpider) scrapeURL(ctx context.Context, url string) error {
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

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("非预期的状态码: %d", resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("读取响应失败: %w", err)
	}

	type APIResponse struct {
		Status string      `json:"status"`
		Data   []ProxyInfo `json:"data"`
		Total  int         `json:"total"`
		Page   int         `json:"page"`
		Limit  int         `json:"limit"`
	}

	var response APIResponse
	if err := json.Unmarshal(body, &response); err != nil {
		log.Printf("解析失败的响应内容: %s", string(body))
		return fmt.Errorf("解析JSON失败: %w", err)
	}

	s.mu.Lock()
	s.results = append(s.results, response.Data...)
	s.mu.Unlock()

	log.Printf("成功爬取 %d 个代理\n", len(response.Data))
	return nil
}

// validateAndDeduplicateResults 验证和去重结果
func (s *GeonodeSpider) validateAndDeduplicateResults() {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 去重
	seen := make(map[string]bool)
	unique := make([]ProxyInfo, 0)
	for _, proxy := range s.results {
		key := fmt.Sprintf("%s:%s", proxy.IP, proxy.Port)
		if !seen[key] && s.validateProxy(proxy) {
			seen[key] = true
			unique = append(unique, proxy)
		}
	}
	s.results = unique
	log.Printf("验证和去重后剩余 %d 个代理\n", len(s.results))
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
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.results) == 0 {
		return fmt.Errorf("没有数据可保存")
	}

	if err := os.MkdirAll("proxies", 0755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}

	filename := fmt.Sprintf("proxies/proxies_%s.json",
		time.Now().Format("20060102_150405"))

	data, err := json.MarshalIndent(s.results, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化结果失败: %w", err)
	}

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

// 添加新的方法来获取总页数
func (s *GeonodeSpider) getTotalPages(ctx context.Context) (int, error) {
	// 构建初始请求URL（只请求第一页）
	url := "https://proxylist.geonode.com/api/proxy-list?limit=100&page=1&sort_by=lastChecked&sort_type=desc&protocols=http%2Chttps"
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return 0, fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("User-Agent", s.getRandomUserAgent())
	req.Header.Set("Accept", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("HTTP请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("读取响应失败: %w", err)
	}

	// 定义响应结构
	type APIResponse struct {
		Total int `json:"total"`
		Limit int `json:"limit"`
	}

	var response APIResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return 0, fmt.Errorf("解析JSON失败: %w", err)
	}

	// 计算总页数（向上取整）
	totalPages := (response.Total + response.Limit - 1) / response.Limit
	log.Printf("总记录数: %d, 每页数量: %d, 总页数: %d\n", 
		response.Total, response.Limit, totalPages)
	
	return totalPages, nil
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	spider := NewGeonodeSpider()

	errChan := make(chan error, 1)
	go func() {
		errChan <- spider.Run(ctx)
	}()

	select {
	case <-sigChan:
		log.Println("收到终止信号，正在优雅关闭...")
		cancel()
		<-errChan
	case err := <-errChan:
		if err != nil {
			log.Fatalf("爬虫运行失败: %v", err)
		}
	}

	log.Println("爬虫已完成")
}
