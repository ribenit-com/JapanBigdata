package spider

import (
	"context"
	"time"
)

// Spider 定义爬虫接口
type Spider interface {
	Init() error
	Process(ctx context.Context, url string) error
	Cleanup() error
}

// BaseSpider 提供基础爬虫实现
type BaseSpider struct {
	Name        string
	Description string
	StartURLs   []string
	Timeout     time.Duration
}

func (s *BaseSpider) Init() error {
	return nil
}

func (s *BaseSpider) Process(ctx context.Context, url string) error {
	return nil
}

func (s *BaseSpider) Cleanup() error {
	return nil
}

func (s *BaseSpider) Run() error {
	ctx, cancel := context.WithTimeout(context.Background(), s.Timeout)
	defer cancel()

	if err := s.Init(); err != nil {
		return err
	}

	for _, url := range s.StartURLs {
		if err := s.Process(ctx, url); err != nil {
			return err
		}
	}

	return s.Cleanup()
}
