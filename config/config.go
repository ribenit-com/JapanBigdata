package config

import (
	"os"

	"gopkg.in/yaml.v2"
)

type Config struct {
	CrawlabHost string `yaml:"crawlab_host"`
	ApiKey      string `yaml:"api_key"`
}

var GlobalConfig Config

func LoadConfig() error {
	data, err := os.ReadFile("config/config.yaml")
	if err != nil {
		return err
	}

	err = yaml.Unmarshal(data, &GlobalConfig)
	if err != nil {
		return err
	}

	return nil
}
