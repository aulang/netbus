package main

import (
	"fmt"
	"github.com/aulang/netbus/config"
	"github.com/aulang/netbus/core"
	"os"
)

func printHelp() {
	fmt.Println(`"-server" 加载 "config.yml" 启动服务端`)
	fmt.Println(`"-client" 加载 "config.yml" 启动客户端`)
	fmt.Println(`"-server <key> <port>" 启动服务端, 监听xxx端口', 如：-server 8888`)
	fmt.Println(`"-client <key> <server:port> <local:port:serverPort>" 启动客户端，如：-client Aulang aulang.cn:8888 127.0.0.1:3306:13306`)
	fmt.Println(`"-generate <key> [expired-time]" 创建一个客户端密钥, 如 -generate Aulang 2020-12-31`)
}

func main() {
	args := os.Args
	argc := len(os.Args)

	if argc < 2 {
		printHelp()
		os.Exit(0)
	}

	// 获取其余参数
	argsConfig := args[2:]

	switch args[1] {
	case "-server":
		// 外网服务
		serverConfig := config.InitServerConfig(argsConfig)
		core.Server(serverConfig)
	case "-client":
		// 内网服务
		clientConfig := config.InitClientConfig(argsConfig)
		core.Client(clientConfig)
	case "-generate":
		// 生成短期 key
		var seed, expired string
		if len(argsConfig) > 0 {
			seed = argsConfig[0]
		}
		if len(argsConfig) > 1 {
			expired = argsConfig[1]
		}
		if len(argsConfig) > 0 {
			trialKey, _ := config.NewKey(seed, expired)
			fmt.Println("客户端密钥 ->	", trialKey)
		}
	default:
		printHelp()
	}
}
