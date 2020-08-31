package core

import (
	"context"
	"github.com/aulang/netbus/config"
	"github.com/lucas-clemente/quic-go"
	"log"
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
func handleClientSession(cfg config.ClientConfig, index int) {
	flagChan := make(chan bool)
	sessionChan := make(chan quic.Session)

	// 远程拨号，建桥
	go buildBridgeSession(cfg, index, sessionChan, flagChan)

	// 本地连接拨号，并建立双向通道
	go buildLocalConnection(cfg.LocalAddr[index], sessionChan, flagChan)

	// 初始化连接
	for i := 0; i < cfg.TunnelCount; i++ {
		flagChan <- true
	}

	log.Printf("初始化通道完成，端口号：[%d]，通道数：[%d]", cfg.LocalAddr[index].Port2, cfg.TunnelCount)
}

func buildBridgeSession(cfg config.ClientConfig, index int, sessionChan chan quic.Session, flagChan chan bool) {
	key := cfg.Key
	serverAddr := cfg.ServerAddr
	localPort := cfg.LocalAddr[index].Port2

	for range flagChan {
		go func(serverAddr config.NetAddress, localPort uint32, key string, sessionChan chan quic.Session, flagChan chan bool) {
			session := dial(serverAddr, 30)
			if session == nil {
				log.Fatalf("向服务器建立连接失败：[%s]", serverAddr.String())
			}

			// 请求建立连接
			if !requestSession(session, key, localPort) {
				log.Println("发送协议数据失败！")
				return
			}

			// 等待服务器端接收数据响应
			resp := receiveProtocol(session)

			// 处理连接结果
			switch resp.Result {
			case protocolResultSuccess:
				// 服务器端接收到数据，准备数据传输
				sessionChan <- session
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
		}(serverAddr, localPort, key, sessionChan, flagChan)
	}
}

// 本地服务连接拨号，并建立双向通道
func buildLocalConnection(local config.NetAddress, sessionChan chan quic.Session, flagChan chan bool) {
	for session := range sessionChan {
		// 建立本地连接访问
		go func(session quic.Session, local config.NetAddress, sessionChan chan quic.Session, flagChan chan bool) {
			// 通知创建新桥
			flagChan <- true

			// 打开流进行数据传输
			serverStream, err := session.AcceptStream(context.Background())
			if err != nil {
				log.Println("打开服务端流失败！", err)
				return
			}

			// 建立本地连接，进行连接数据传输
			if localConn := tcpDial(local, 5); localConn != nil {
				// 进行数据传输
				forward(serverStream, localConn)
			} else {
				// 打开本地连接失败，关闭服务器流
				closeWithoutError(serverStream)
				// 放弃连接，重新建桥
				log.Println("打开本地连接失败！")
			}
		}(session, local, sessionChan, flagChan)
	}
}

// 入口
func Client(cfg config.ClientConfig) {
	log.Println("加载配置：", cfg)

	// 遍历所有端口建桥
	for index := range cfg.LocalAddr {
		go handleClientSession(cfg, index)
	}

	select {}
}
