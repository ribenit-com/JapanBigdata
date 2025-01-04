package spider

import (
	"context"
	"time"
)

// Spider 定义爬虫接口
// 所有具体的爬虫实现都需要满足这个接口
type Spider interface {
	// Init 初始化爬虫
	// 在爬虫开始工作前执行，用于设置初始状态
	Init() error

	// Process 处理单个URL
	// ctx: 用于控制爬虫生命周期的上下文
	// url: 需要处理的目标URL
	Process(ctx context.Context, url string) error

	// Cleanup 清理资源
	// 在爬虫工作结束后执行，用于释放资源
	Cleanup() error
}

// BaseSpider 提供基础爬虫实现
// 包含所有爬虫通用的属性和方法
type BaseSpider struct {
	Name        string        // 爬虫名称，用于标识和日志
	Description string        // 爬虫描述，说明爬虫的用途
	StartURLs   []string      // 起始URL列表，爬虫从这些URL开始工作
	Timeout     time.Duration // 爬虫超时时间，防止程序无限运行
}

// Init 基础初始化实现
// 可被具体爬虫重写以添加自定义初始化逻辑
func (s *BaseSpider) Init() error {
	return nil
}

// Process 基础URL处理实现
// 可被具体爬虫重写以实现实际的爬取逻辑
func (s *BaseSpider) Process(ctx context.Context, url string) error {
	return nil
}

// Cleanup 基础清理实现
// 可被具体爬虫重写以添加自定义清理逻辑
func (s *BaseSpider) Cleanup() error {
	return nil
}

// Run 运行爬虫的主要逻辑
// 按顺序执行初始化、URL处理和清理工作
func (s *BaseSpider) Run() error {
	// 创建带超时的上下文
	ctx, cancel := context.WithTimeout(context.Background(), s.Timeout)
	defer cancel() // 确保资源被释放

	// 执行初始化
	if err := s.Init(); err != nil {
		return err
	}

	// 处理所有起始URL
	for _, url := range s.StartURLs {
		// 检查上下文是否被取消
		if err := s.Process(ctx, url); err != nil {
			return err
		}
	}

	// 执行清理工作
	return s.Cleanup()
}
