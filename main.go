package main

import (
	"flag"
	"fmt"
	"github.com/aulang/netbus/config"
	"github.com/aulang/netbus/core"
)

var server = flag.Bool("server", false, "启动服务端")
var client = flag.Bool("client", false, "启动客户端")
var generate = flag.Bool("generate", false, "创建客户端密钥")

func printHelp() {
	fmt.Println(`"-server" 加载 "config.yml" 启动服务端`)
	fmt.Println(`"-client" 加载 "config.yml" 启动客户端`)
	fmt.Println(`"-server <key> <port>" 启动服务端, 监听xxx端口', 如：-server 8888`)
	fmt.Println(`"-client <key> <server:port> <local:port:serverPort>" 启动客户端，如：-client Aulang aulang.cn:8888 127.0.0.1:3306:13306`)
	fmt.Println(`"-generate <key> [expired-time]" 创建客户端密钥, 如 -generate Aulang 2020-12-31`)
}

func main() {
	flag.Parse()
	// 获取其余参数
	argsConfig := flag.Args()

	if *server {
		serverConfig := config.InitServerConfig(argsConfig)
		core.Server(serverConfig)
	} else if *client {
		clientConfig := config.InitClientConfig(argsConfig)
		core.Client(clientConfig)
	} else if *generate {
		var seed, expired string
		if len(argsConfig) > 0 {
			seed = argsConfig[0]
		}
		if len(argsConfig) > 1 {
			expired = argsConfig[1]
		}
		if len(argsConfig) > 0 {
			trialKey, _ := config.NewKey(seed, expired)
			fmt.Printf("客户端密钥：%s\n", trialKey)
		}
	} else {
		printHelp()
	}
}
