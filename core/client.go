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
	serverAddr := cfg.ServerAddr
	localAddr := cfg.LocalAddr[index]

	for {
		select {
		case <-flagChan:
			// 新建协程，向桥端建立连接
			go func(ch chan quic.Session) {
				session := dial(serverAddr, 30)
				if session == nil {
					log.Fatalf("向服务器建立连接失败：[%s]", serverAddr.String())
				}

				// 请求建立连接
				if !requestSession(session, cfg.Key, localAddr.Port2) {
					log.Println("发送协议数据失败！")
					return
				}

				resp := receiveProtocol(session)

				// 处理连接结果
				switch resp.Result {
				case protocolResultSuccess:
					ch <- session
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
					// 连接中断
					log.Println("连接中断，重新连接！")
					// 重新连接
					flagChan <- true
				}
			}(sessionChan)
		}
	}
}

// 本地服务连接拨号，并建立双向通道
func buildLocalConnection(local config.NetAddress, sessionChan chan quic.Session, flagChan chan bool) {
	for {
		select {
		case session := <-sessionChan:
			// 建立本地连接访问
			go func(serverSession quic.Session) {
				// 通知创建新桥
				flagChan <- true
				if localConn := tcpDial(local, 5); localConn != nil {
					// 打开流进行数据传输
					serverStream, err := serverSession.AcceptStream(context.Background())
					if err != nil {
						log.Printf("打开服务端流失败，关闭本地连接：[%s]\n", localConn.RemoteAddr().String())
						closeWithoutError(localConn)
						return
					}

					log.Println("进行连接数据传输！")
					forward(serverStream, localConn)
				} else {
					// 放弃连接，重新建桥
					log.Println("打开本地连接失败！")
				}
			}(session)
		}
	}
}

// 入口
func Client(cfg config.ClientConfig) {
	log.Println("加载配置：", cfg)

	// 遍历所有端口
	for index := range cfg.LocalAddr {
		go handleClientSession(cfg, index)
	}
	select {}
}
