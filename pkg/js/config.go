package js

import "time"

// Config JS渲染控制器配置
type Config struct {
	PoolSize        int           // 浏览器实例池大小
	PageTimeout     time.Duration // 页面加载超时时间
	MetricsInterval time.Duration // 指标收集间隔
}

// RenderOptions 渲染选项
type RenderOptions struct {
	WaitSelector string // 等待选择器
	Script       string // 自定义脚本
	Screenshot   bool   // 是否截图
}

// RenderResult 渲染结果
type RenderResult struct {
	HTML       string // 页面HTML内容
	Screenshot string // Base64编码的截图
}
