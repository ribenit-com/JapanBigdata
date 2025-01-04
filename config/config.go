package config

import (
	"os"

	"gopkg.in/yaml.v2"
)

// Config 定义配置文件结构
type Config struct {
	CrawlabHost string `yaml:"crawlab_host"`
	ApiKey      string `yaml:"api_key"`
	Log         struct {
		Level string `yaml:"level"`
		File  string `yaml:"file"`
	} `yaml:"log"`
	Spider struct {
		Timeout    int `yaml:"timeout"`
		RetryCount int `yaml:"retry_count"`
	} `yaml:"spider"`
	Node struct {
		MaxTasks int `yaml:"max_tasks"`
	} `yaml:"node"`
	Misc struct {
		EnableDebug bool `yaml:"enable_debug"`
	} `yaml:"misc"`
}

var GlobalConfig Config

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

	// 创建日志目录
	logDir := "logs"
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return err
	}

	return nil
}
