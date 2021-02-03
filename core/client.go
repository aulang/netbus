package core

import (
	"github.com/aulang/netbus/config"
	"log"
	"net"
	"sync"
)

// 发送代理请求
func sendProxyRequest(conn net.Conn, key string, port uint32) bool {
	request := Protocol{
		Result:  protocolResultSuccess,
		Version: protocolVersion,
		Port:    port,
		Key:     key,
	}

	return sendProtocol(conn, request)
}

// 打开服务端代理连接
func buildServerTunnel(cfg config.ClientConfig, proxyAddr config.NetAddress, wg *sync.WaitGroup) {
	flagChan := make(chan bool)

	// 远程拨号，建立代理会话
	go openProxyConn(cfg, proxyAddr, flagChan, wg)

	// 初始化连接
	for i := 0; i < cfg.TunnelCount; i++ {
		flagChan <- true
	}

	log.Printf("初始化通道完成，本地端口：[%d]，服务器代理端口号：[%d]，通道数：[%d]\n",
		proxyAddr.Port,
		proxyAddr.ProxyPort,
		cfg.TunnelCount)
}

func openProxyConn(cfg config.ClientConfig, proxyAddr config.NetAddress, flagChan chan bool, wg *sync.WaitGroup) {
	key := cfg.Key
	serverAddr := cfg.ServerAddr

	for flag := range flagChan {
		if !flag {
			// 不再创建新桥
			wg.Done()
			return
		}

		go func(serverAddr config.NetAddress, proxyAddr config.NetAddress, key string, flagChan chan bool) {
			serverConn := dial(serverAddr, 10)
			if serverConn == nil {
				log.Fatalf("向服务器建立连接失败：[%s]\n", serverAddr.String())
			}

			// 请求建立连接
			if !sendProxyRequest(serverConn, key, proxyAddr.ProxyPort) {
				log.Println("发送协议数据失败！")
				closeWithoutError(serverConn)
				// 重新连接
				flagChan <- true
				return
			}

			// 等待服务器端接收数据响应
			protocol := receiveProtocol(serverConn)

			// 处理连接结果
			switch protocol.Result {
			case protocolResultSuccess:
				// 接收到服务器端数据，准备数据传输
				go receiveData(proxyAddr, serverConn, flagChan)
			case protocolResultVersionMismatch:
				closeWithoutError(serverConn)
				// 版本不匹配，退出客户端
				log.Fatalln("版本不匹配！")
			case protocolResultFailToAuth:
				closeWithoutError(serverConn)
				// 鉴权失败，退出客户端
				log.Fatalln("认证失败！")
			case protocolResultIllegalAccessPort:
				closeWithoutError(serverConn)
				// 访问端口不合法，退出客户端
				log.Fatalln("端口不合法！")
			default:
				closeWithoutError(serverConn)
				// 连接中断，重新连接
				flagChan <- true
			}
		}(serverAddr, proxyAddr, key, flagChan)
	}
}

// 本地服务连接拨号，并建立双向通道
func receiveData(proxyAddr config.NetAddress, serverConn net.Conn, flagChan chan bool) {
	// 建立本地连接，进行连接数据传输
	if localConn := dial(proxyAddr, 5); localConn != nil {
		flagChan <- true
		forward(serverConn, localConn)
	} else {
		log.Printf("本地端口 [%d] 服务已停止！\n", proxyAddr.Port)
		// 打开本地连接失败，关闭服务器流
		closeWithoutError(serverConn)
		flagChan <- false
	}
}

// 入口
func Client(cfg config.ClientConfig) {
	log.Println("加载配置：", cfg)

	var wg sync.WaitGroup

	// 遍历所有代理地址配置，建立代理连接
	for _, proxyAddr := range cfg.ProxyAddrs {
		wg.Add(1)

		go buildServerTunnel(cfg, proxyAddr, &wg)
	}

	wg.Wait()
}
