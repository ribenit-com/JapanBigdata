package config

import (
	"os"

	"gopkg.in/yaml.v2"
)

// Config 定义配置文件结构
// 使用 yaml 标签来映射配置文件中的键名
type Config struct {
	CrawlabHost string `yaml:"crawlab_host"` // Crawlab 服务器地址
	ApiKey      string `yaml:"api_key"`      // API 认证密钥
}

// GlobalConfig 全局配置实例，其他包可以通过此变量访问配置
var GlobalConfig Config

// LoadConfig 从配置文件加载配置信息
// 读取 config/config.yaml 文件并解析到 GlobalConfig 中
func LoadConfig() error {
	// 读取配置文件
	data, err := os.ReadFile("config/config.yaml")
	if err != nil {
		return err
	}

	// 解析 YAML 到结构体
	err = yaml.Unmarshal(data, &GlobalConfig)
	if err != nil {
		return err
	}

	return nil
}
