package config

import (
	"log"
	"strconv"
	"strings"
)

const (
	// 默认最大隧道数
	minTunnelCount = 1
	maxTunnelCount = 10
)

// 客户端配置
type ClientConfig struct {
	Key         string       // 参考服务端配置
	ServerAddr  NetAddress   // 服务端地址
	ProxyAddrs  []NetAddress // 内网服务地址及映射端口
	TunnelCount int          // 隧道条数(1-10)
}

var clientConfig ClientConfig

// 从参数中解析配置
func parseClientConfig(args []string) ClientConfig {
	if len(args) < 3 {
		log.Fatalln("参数缺失。", args)
	}

	config := ClientConfig{TunnelCount: minTunnelCount}
	var ok bool

	// 1 Key
	config.Key = strings.TrimSpace(args[0])
	// 2 ServerAddr
	if config.ServerAddr, ok = ParseNetAddress(strings.TrimSpace(args[1])); !ok {
		log.Fatalln("服务端地址错误。", args[1])
	}
	// 3 ProxyAddrs
	if config.ProxyAddrs, ok = ParseNetAddresses(strings.TrimSpace(args[2])); !ok {
		log.Fatalln("内网服务地址及映射端口错误。", args[2])
	}
	// 4 TunnelCount
	if len(args) >= 4 {
		var err error
		if config.TunnelCount, err = strconv.Atoi(args[3]); err != nil {
			log.Fatalln("隧道条数错误", args[3])
		}
		if config.TunnelCount > maxTunnelCount {
			config.TunnelCount = maxTunnelCount
		}
		if config.TunnelCount < minTunnelCount {
			config.TunnelCount = minTunnelCount
		}
	}
	return config
}

// 从配置文件中加载配置
func loadClientConfig() ClientConfig {
	config := ClientConfig{}

	config.Key = Config.Client.Key

	var ok bool
	if config.ServerAddr, ok = ParseNetAddress(Config.Client.ServerAddr); !ok {
		log.Fatalln("服务端地址配置错误。", Config.Client.ServerAddr)
	}

	for _, proxyMapping := range Config.Client.ProxyMappings {
		if proxyAddr, ok := ParseNetAddress(proxyMapping); ok {
			config.ProxyAddrs = append(config.ProxyAddrs, proxyAddr)
		} else {
			log.Println("内网服务地址及映射端口配置错误。", proxyMapping)
		}
	}

	if len(config.ProxyAddrs) < 1 {
		log.Fatalln("内网服务地址及映射端口配置错误。")
	}

	config.TunnelCount = Config.Client.TunnelCount
	if config.TunnelCount > maxTunnelCount {
		config.TunnelCount = maxTunnelCount
	}
	if config.TunnelCount < minTunnelCount {
		config.TunnelCount = minTunnelCount
	}

	return config
}

// 初始化客户端配置，支持从参数中读取或者从配置文件中读取
func InitClientConfig(args []string) ClientConfig {
	if len(args) == 0 {
		LoadConfigFile()
		clientConfig = loadClientConfig()
	} else {
		clientConfig = parseClientConfig(args)
	}
	return clientConfig
}
