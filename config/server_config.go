package config

import (
	"log"
	"strings"
)

// 服务端配置
type ServerConfig struct {
	Port          uint32 // 服务端口
	Key           string // 6-16 个字符，用于身份校验
	MinAccessPort uint32 // 最小访问端口，最小值 1024
	MaxAccessPort uint32 // 最大访问端口，最大值 65535
}

// 检查端口是否在允许范围内，不含边界
func (c *ServerConfig) PortInRange(port uint32) bool {
	return port > c.MinAccessPort && port < c.MaxAccessPort
}

var serverConfig ServerConfig

// 从参数中解析配置
func parseServerConfig(args []string) ServerConfig {
	if len(args) < 2 {
		log.Fatalln("参数缺失！", args)
	}
	// 0 key
	key := strings.TrimSpace(args[0])

	// 1 port
	port, err := parsePort(args[1])
	if err != nil || !checkPort(port) {
		log.Fatalln("端口号错误。", args[1])
	}

	// 2 access port range
	portRange := strings.Split(args[2], "-")
	if len(portRange) != 2 {
		log.Fatalln("访问端口号范围错误。", args[2])
	}

	minAccessPort, err := parsePort(portRange[0])
	if err != nil || !checkPort(minAccessPort) {
		log.Fatalln("最小访问端口号错误。", portRange[0])
	}
	maxAccessPort, err := parsePort(portRange[1])
	if err != nil || !checkPort(maxAccessPort) {
		log.Fatalln("最小访问端口号错误。", portRange[1])
	}
	// 检查范围是否正确，确保范围内至少有一个元素
	if maxAccessPort-minAccessPort < 2 {
		log.Fatalln("访问端口号范围错误。", args[2])
	}

	return ServerConfig{
		Port:          port,
		Key:           key,
		MinAccessPort: minAccessPort,
		MaxAccessPort: maxAccessPort,
	}
}

// 从配置文件中加载配置
func loadServerConfig() ServerConfig {
	if !checkPort(Config.Server.Port) {
		log.Fatalln("端口号配置错误。", Config.Server.Port)
	}

	if !checkPort(Config.Server.MinAccessPort) {
		log.Fatalln("最小访问端口号配置错误。", Config.Server.MinAccessPort)
	}

	if !checkPort(Config.Server.MaxAccessPort) {
		log.Fatalln("最大访问端口配置错误。", Config.Server.MaxAccessPort)
	}

	if Config.Server.MaxAccessPort-Config.Server.MinAccessPort < 2 {
		log.Fatalln("访问端口号范围错误。", Config.Server.MinAccessPort, Config.Server.MaxAccessPort)
	}

	return ServerConfig{
		Port:          Config.Server.Port,
		Key:           Config.Server.Key,
		MinAccessPort: Config.Server.MinAccessPort,
		MaxAccessPort: Config.Server.MaxAccessPort,
	}
}

// 初始化服务端配置，支持从参数中读取或者从配置文件中读取
func InitServerConfig(args []string) ServerConfig {
	if len(args) == 0 {
		serverConfig = loadServerConfig()
	} else {
		serverConfig = parseServerConfig(args)
	}
	return serverConfig
}
