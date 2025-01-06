package js

import (
	"context"
	"encoding/base64"
	"fmt"
	"sync"
	"time"

	"github.com/chromedp/chromedp"
)

// JSController JavaScript渲染控制器
type JSController struct {
	config  Config
	pool    *BrowserPool
	metrics *Metrics
}

// BrowserPool 浏览器实例池
type BrowserPool struct {
	contexts []context.Context
	current  int
	size     int
	mu       sync.Mutex
}

// Metrics 性能指标
type Metrics struct {
	TotalRequests   int64
	SuccessRequests int64
	FailedRequests  int64
	AverageLoadTime time.Duration
	mu              sync.Mutex
}

// NewJSController 创建新的JS渲染控制器
func NewJSController(config Config) (*JSController, error) {
	pool, err := newBrowserPool(config.PoolSize)
	if err != nil {
		return nil, err
	}

	jc := &JSController{
		config:  config,
		pool:    pool,
		metrics: &Metrics{},
	}

	// 启动指标收集
	go jc.startMetricsCollector()

	return jc, nil
}

// RenderPage 渲染页面并获取内容
func (jc *JSController) RenderPage(ctx context.Context, url string, opts *RenderOptions) (*RenderResult, error) {
	start := time.Now()
	defer func() {
		jc.updateMetrics(time.Since(start))
	}()

	// 从池中获取浏览器上下文
	browserCtx, err := jc.pool.acquire()
	if err != nil {
		return nil, err
	}
	defer jc.pool.release(browserCtx)

	// 设置超时
	timeoutCtx, cancel := context.WithTimeout(ctx, jc.config.PageTimeout)
	defer cancel()

	// 准备任务列表
	tasks := []chromedp.Action{
		chromedp.Navigate(url),
		chromedp.WaitReady("body", chromedp.ByQuery),
	}

	// 添加等待选择器
	if opts.WaitSelector != "" {
		tasks = append(tasks, chromedp.WaitVisible(opts.WaitSelector, chromedp.ByQuery))
	}

	// 添加自定义脚本
	if opts.Script != "" {
		tasks = append(tasks, chromedp.Evaluate(opts.Script, nil))
	}

	// 执行渲染任务
	if err := chromedp.Run(timeoutCtx, tasks...); err != nil {
		return nil, fmt.Errorf("render failed: %w", err)
	}

	// 获取渲染结果
	result := &RenderResult{}

	// 获取HTML内容
	if err := chromedp.Run(timeoutCtx, chromedp.OuterHTML("html", &result.HTML)); err != nil {
		return nil, fmt.Errorf("get html failed: %w", err)
	}

	// 如果需要截图
	if opts.Screenshot {
		var buf []byte
		if err := chromedp.Run(timeoutCtx, chromedp.CaptureScreenshot(&buf)); err != nil {
			return nil, fmt.Errorf("screenshot failed: %w", err)
		}
		result.Screenshot = base64.StdEncoding.EncodeToString(buf)
	}

	return result, nil
}

// ExecuteScript 执行JavaScript脚本
func (jc *JSController) ExecuteScript(ctx context.Context, url, script string) (interface{}, error) {
	browserCtx, err := jc.pool.acquire()
	if err != nil {
		return nil, err
	}
	defer jc.pool.release(browserCtx)

	timeoutCtx, cancel := context.WithTimeout(ctx, jc.config.PageTimeout)
	defer cancel()

	var result interface{}
	err = chromedp.Run(timeoutCtx,
		chromedp.Navigate(url),
		chromedp.WaitReady("body"),
		chromedp.Evaluate(script, &result),
	)

	return result, err
}

// SimulateEvent 模拟事件触发
func (jc *JSController) SimulateEvent(ctx context.Context, url, selector, eventType string) error {
	browserCtx, err := jc.pool.acquire()
	if err != nil {
		return err
	}
	defer jc.pool.release(browserCtx)

	timeoutCtx, cancel := context.WithTimeout(ctx, jc.config.PageTimeout)
	defer cancel()

	return chromedp.Run(timeoutCtx,
		chromedp.Navigate(url),
		chromedp.WaitVisible(selector),
		chromedp.Evaluate(fmt.Sprintf(`
			document.querySelector('%s').dispatchEvent(new Event('%s'))
		`, selector, eventType), nil),
	)
}

// newBrowserPool 创建浏览器实例池
func newBrowserPool(size int) (*BrowserPool, error) {
	pool := &BrowserPool{
		contexts: make([]context.Context, size),
		size:     size,
	}

	// 创建浏览器实例
	for i := 0; i < size; i++ {
		ctx, _ := chromedp.NewContext(context.Background())
		pool.contexts[i] = ctx
	}

	return pool, nil
}

// acquire 获取浏览器实例
func (p *BrowserPool) acquire() (context.Context, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	ctx := p.contexts[p.current]
	p.current = (p.current + 1) % p.size
	return ctx, nil
}

// release 释放浏览器实例
func (p *BrowserPool) release(context.Context) {
	// 实现实例重置或清理逻辑
}

// startMetricsCollector 启动指标收集器
func (jc *JSController) startMetricsCollector() {
	ticker := time.NewTicker(jc.config.MetricsInterval)
	defer ticker.Stop()

	for range ticker.C {
		jc.metrics.mu.Lock()
		// 更新成功率
		if jc.metrics.TotalRequests > 0 {
			jc.metrics.SuccessRequests = jc.metrics.TotalRequests - jc.metrics.FailedRequests
		}
		jc.metrics.mu.Unlock()
	}
}

// updateMetrics 更新性能指标
func (jc *JSController) updateMetrics(duration time.Duration) {
	jc.metrics.mu.Lock()
	defer jc.metrics.mu.Unlock()

	jc.metrics.TotalRequests++
	jc.metrics.AverageLoadTime = time.Duration(
		(int64(jc.metrics.AverageLoadTime)*jc.metrics.TotalRequests + int64(duration)) /
			(jc.metrics.TotalRequests + 1))
}

// GetMetrics 获取性能指标
func (jc *JSController) GetMetrics() *Metrics {
	jc.metrics.mu.Lock()
	defer jc.metrics.mu.Unlock()
	return jc.metrics
}
