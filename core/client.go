package core

import (
	"context"
	"github.com/aulang/netbus/config"
	"github.com/lucas-clemente/quic-go"
	"log"
	"sync"
)

// 发送代理请求
func sendProxyRequest(session quic.Session, key string, port uint32) bool {
	request := Protocol{
		Result:  protocolResultSuccess,
		Version: protocolVersion,
		Port:    port,
		Key:     key,
	}

	return sendProtocol(session, request)
}

// 打开服务端代理连接
func openProxyConnection(cfg config.ClientConfig, proxyAddr config.NetAddress, wg *sync.WaitGroup) {
	flagChan := make(chan bool)

	// 远程拨号，建立代理会话
	go openProxySession(cfg, proxyAddr, flagChan, wg)

	// 初始化连接
	for i := 0; i < cfg.TunnelCount; i++ {
		flagChan <- true
	}

	log.Printf("初始化通道完成，本地端口：[%d]，服务器代理端口号：[%d]，通道数：[%d]\n",
		proxyAddr.Port,
		proxyAddr.ProxyPort,
		cfg.TunnelCount)
}

func openProxySession(cfg config.ClientConfig, proxyAddr config.NetAddress, flagChan chan bool, wg *sync.WaitGroup) {
	key := cfg.Key
	serverAddr := cfg.ServerAddr

	for flag := range flagChan {
		if !flag {
			// 不再创建新桥
			wg.Done()
			return
		}

		go func(serverAddr config.NetAddress, proxyAddr config.NetAddress, key string, flagChan chan bool) {
			serverSession := dial(serverAddr, 10)
			if serverSession == nil {
				log.Fatalf("向服务器建立连接失败：[%s]\n", serverAddr.String())
			}

			// 请求建立连接
			if !sendProxyRequest(serverSession, key, proxyAddr.ProxyPort) {
				log.Println("发送协议数据失败！")
				// 重新连接
				flagChan <- true
				return
			}

			// 等待服务器端接收数据响应
			protocol := receiveProtocol(serverSession)

			// 处理连接结果
			switch protocol.Result {
			case protocolResultSuccess:
				// 接收到服务器端数据，准备数据传输
				recvData(proxyAddr, serverSession, flagChan)
			case protocolResultVersionMismatch:
				// 版本不匹配，退出客户端
				log.Fatalln("版本不匹配！")
			case protocolResultFailToAuth:
				// 鉴权失败，退出客户端
				log.Fatalln("认证失败！")
			case protocolResultIllegalAccessPort:
				// 访问端口不合法，退出客户端
				log.Fatalln("端口不合法！")
			default:
				// 连接中断，重新连接
				flagChan <- true
			}
		}(serverAddr, proxyAddr, key, flagChan)
	}
}

// 本地服务连接拨号，并建立双向通道
func recvData(proxyAddr config.NetAddress, serverSession quic.Session, flagChan chan bool) {
	// 打开流进行数据传输
	serverStream, err := serverSession.AcceptStream(context.Background())
	if err != nil {
		log.Println("打开服务端流失败！", err)
		flagChan <- true
		return
	}

	// 建立本地连接，进行连接数据传输
	if localConn := tcpDial(proxyAddr, 5); localConn != nil {
		flagChan <- true
		forward(serverStream, localConn)
	} else {
		log.Printf("本地端口 [%d] 服务已停止！\n", proxyAddr.Port)
		// 打开本地连接失败，关闭服务器流
		closeWithoutError(serverStream)
		flagChan <- true
	}
}

// 入口
func Client(cfg config.ClientConfig) {
	log.Println("加载配置：", cfg)

	var wg sync.WaitGroup

	wg.Add(len(cfg.ProxyAddrs))

	// 遍历所有代理地址配置，建立代理连接
	for _, proxyAddr := range cfg.ProxyAddrs {
		go openProxyConnection(cfg, proxyAddr, &wg)
	}

	wg.Wait()
}
