package spider

import (
	"sync"
)

// SpiderRegistry 爬虫注册中心
type SpiderRegistry struct {
	spiders map[string]Spider
	mu      sync.RWMutex
}

// RegisterSpider 注册一个新的爬虫
func (r *SpiderRegistry) RegisterSpider(name string, spider Spider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.spiders[name] = spider
}

// GetSpider 获取指定名称的爬虫
func (r *SpiderRegistry) GetSpider(name string) (Spider, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	spider, exists := r.spiders[name]
	return spider, exists
}
