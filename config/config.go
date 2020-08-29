package config

import (
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
)

type Yaml struct {
	Server struct {
		Key           string `yaml:"key"`
		Port          uint32 `yaml:"port"`
		MinAccessPort uint32 `yaml:"min-access-port"`
		MaxAccessPort uint32 `yaml:"max-access-port"`
	}
	Client struct {
		Key              string   `yaml:"key"`
		ServerHost       string   `yaml:"server-host"`
		LocalHostMapping []string `yaml:"local-host-mapping"`
		TunnelCount      int      `yaml:"tunnel-count"`
	}
}

var Config = new(Yaml)

func init() {
	configFile, err := ioutil.ReadFile("config.yml")

	if err != nil {
		log.Fatalf("加载配置文件失败，%v", err)
	}

	err = yaml.Unmarshal(configFile, Config)

	if err != nil {
		log.Fatalf("解析配置文件失败: %v", err)
	}
}
