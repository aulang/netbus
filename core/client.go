package core

import (
	"context"
	"github.com/aulang/netbus/config"
	"github.com/lucas-clemente/quic-go"
	"log"
	"sync"
)

// 请求连接
func requestSession(session quic.Session, key string, port uint32) bool {
	request := Protocol{
		Result:  protocolResultSuccess,
		Version: protocolVersion,
		Port:    port,
		Key:     key,
	}

	return sendProtocol(session, request)
}

// 处理客户端连接
func handleClientSession(cfg config.ClientConfig, localAddr config.NetAddress, wg *sync.WaitGroup) {
	flagChan := make(chan bool)

	// 远程拨号，建桥
	go buildBridgeSession(cfg, localAddr, flagChan, wg)

	// 初始化连接
	for i := 0; i < cfg.TunnelCount; i++ {
		flagChan <- true
	}

	log.Printf("初始化通道完成，端口号：[%d]，通道数：[%d]", localAddr.Port2, cfg.TunnelCount)
}

func buildBridgeSession(cfg config.ClientConfig, localAddr config.NetAddress, flagChan chan bool, wg *sync.WaitGroup) {
	key := cfg.Key
	serverAddr := cfg.ServerAddr

	for flag := range flagChan {
		if !flag {
			// 不再创建新桥
			wg.Done()
			return
		}

		go func(serverAddr config.NetAddress, localAddr config.NetAddress, key string, flagChan chan bool) {
			session := dial(serverAddr, 30)
			if session == nil {
				log.Fatalf("向服务器建立连接失败：[%s]", serverAddr.String())
			}

			// 请求建立连接
			if !requestSession(session, key, localAddr.Port2) {
				log.Println("发送协议数据失败！")
				return
			}

			// 等待服务器端接收数据响应
			resp := receiveProtocol(session)

			// 处理连接结果
			switch resp.Result {
			case protocolResultSuccess:
				// 接收到服务器端数据，准备数据传输
				go handleConnection(localAddr, session, flagChan)
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
		}(serverAddr, localAddr, key, flagChan)
	}
}

// 本地服务连接拨号，并建立双向通道
func handleConnection(localAddr config.NetAddress, session quic.Session, flagChan chan bool) {
	// 打开流进行数据传输
	serverStream, err := session.AcceptStream(context.Background())
	if err != nil {
		log.Println("打开服务端流失败！", err)
		// 通知创建新桥
		flagChan <- true
		return
	}

	// 建立本地连接，进行连接数据传输
	if localConn := tcpDial(localAddr, 5); localConn != nil {
		// 通知创建新桥
		flagChan <- true
		forward(serverStream, localConn)
	} else {
		log.Println("本地端口服务已停止！")
		// 打开本地连接失败，关闭服务器流
		closeWithoutError(serverStream)
		// 不再创建该端口新桥
		flagChan <- false
	}
}

// 入口
func Client(cfg config.ClientConfig) {
	log.Println("加载配置：", cfg)

	var wg sync.WaitGroup

	wg.Add(len(cfg.LocalAddr))

	// 遍历所有端口建桥
	for _, localAddr := range cfg.LocalAddr {
		go handleClientSession(cfg, localAddr, &wg)
	}

	wg.Wait()
}
