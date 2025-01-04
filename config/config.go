package config

import (
	"os"

	"gopkg.in/yaml.v2"
)

type Config struct {
	App struct {
		Name    string `yaml:"name"`
		Version string `yaml:"version"`
	} `yaml:"app"`

	Spider struct {
		Timeout  int `yaml:"timeout"`
		MaxRetry int `yaml:"max_retry"`
	} `yaml:"spider"`

	Node struct {
		Master            string `yaml:"master"`
		HeartbeatInterval int    `yaml:"heartbeat_interval"`
	} `yaml:"node"`

	Log struct {
		Level string `yaml:"level"`
		Path  string `yaml:"path"`
	} `yaml:"log"`
}

var GlobalConfig Config

func Init() error {
	f, err := os.Open("config/config.yml")
	if err != nil {
		return err
	}
	defer f.Close()

	decoder := yaml.NewDecoder(f)
	err = decoder.Decode(&GlobalConfig)
	if err != nil {
		return err
	}

	return nil
}
