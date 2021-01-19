package config

import (
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
)

type Yaml struct {
	Server struct {
		Key          string `yaml:"key"`
		Port         uint32 `yaml:"port"`
		MinProxyPort uint32 `yaml:"min-proxy-port"`
		MaxProxyPort uint32 `yaml:"max-proxy-port"`
	}
	Client struct {
		Key           string   `yaml:"key"`
		ServerAddr    string   `yaml:"server-addr"`
		ProxyMappings []string `yaml:"proxy-mappings"`
		TunnelCount   int      `yaml:"tunnel-count"`
	}
}

var Config = new(Yaml)

func LoadConfigFile() {
	// 获取可执行文件相对于当前工作目录的相对路径
	root := filepath.Dir(os.Args[0])

	// 根据相对路径获取可执行文件的绝对路径
	root, err := filepath.Abs(root)
	if err != nil {
		log.Fatalf("加载配置文件失败，%v\n", err)
	}

	configFilePath := root + string(filepath.Separator) + "config.yml"

	configFile, err := ioutil.ReadFile(configFilePath)

	if err != nil {
		log.Fatalf("加载配置文件失败，%v\n", err)
	}

	err = yaml.Unmarshal(configFile, Config)

	if err != nil {
		log.Fatalf("解析配置文件失败: %v\n", err)
	}
}
