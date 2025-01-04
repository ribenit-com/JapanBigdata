package config

import (
	"os"

	"gopkg.in/yaml.v2"
)

// Config 定义配置文件结构
// 使用 yaml 标签将结构体字段映射到配置文件中的键
type Config struct {
	// Crawlab服务相关配置
	CrawlabHost string `yaml:"crawlab_host"` // Crawlab服务器地址
	ApiKey      string `yaml:"api_key"`      // API访问密钥

	// 日志相关配置
	Log struct {
		Level string `yaml:"level"` // 日志级别
		File  string `yaml:"file"`  // 日志文件路径
	} `yaml:"log"`

	// 爬虫相关配置
	Spider struct {
		Timeout    int `yaml:"timeout"`     // 爬虫超时时间（秒）
		RetryCount int `yaml:"retry_count"` // 失败重试次数
	} `yaml:"spider"`

	// 节点相关配置
	Node struct {
		MaxTasks int `yaml:"max_tasks"` // 最大并发任务数
	} `yaml:"node"`

	// 其他配置
	Misc struct {
		EnableDebug bool `yaml:"enable_debug"` // 是否启用调试模式
	} `yaml:"misc"`
}

// GlobalConfig 全局配置实例
// 其他包可以通过此变量访问配置信息
var GlobalConfig Config

// LoadConfig 加载并解析配置文件
// 返回错误信息，如果加载或解析失败
func LoadConfig() error {
	// 读取配置文件内容
	data, err := os.ReadFile("config/config.yaml")
	if err != nil {
		return err
	}

	// 将YAML内容解析到结构体
	err = yaml.Unmarshal(data, &GlobalConfig)
	if err != nil {
		return err
	}

	// 创建日志目录，确保日志文件可以正确创建
	logDir := "logs"
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return err
	}

	return nil
}
