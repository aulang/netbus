package core

import (
	"github.com/aulang/netbus/config"
	"log"
	"net"
	"runtime"
)

// 请求连接
func requestConnection(serverConn net.Conn, key string, port uint32) Protocol {
	request := Protocol{
		Result:  protocolResultSuccess,
		Version: protocolVersion,
		Port:    port,
		Key:     key,
	}
	if !sendProtocol(serverConn, request) {
		return request.NewResult(protocolResultFailToSend)
	}
	return receiveProtocol(serverConn)
}

// 处理客户端连接
func handleClientConnection(cfg config.ClientConfig, index int) {
	connChan := make(chan net.Conn)
	flagChan := make(chan bool)

	// 远程拨号，建桥
	go buildBridgeConnection(cfg, index, connChan, flagChan)

	// 本地连接拨号，并建立双向通道
	go buildLocalConnection(cfg.LocalAddr[index], connChan, flagChan)

	// 初始化连接
	for i := 0; i < cfg.TunnelCount; i++ {
		flagChan <- true
	}
	log.Printf("初始化通道失败 [%d] [%d]", cfg.LocalAddr[index].Port2, cfg.TunnelCount)
}

func buildBridgeConnection(cfg config.ClientConfig, index int, connChan chan net.Conn, flagCh chan bool) {
	server := cfg.ServerAddr
	local := cfg.LocalAddr[index]

	for {
		select {
		case <-flagCh:
			// 新建协程，向桥端建立连接
			go func(ch chan net.Conn) {
				conn := dial(server, -1)
				if conn == nil {
					runtime.Goexit()
				}
				// 此处会阻塞，以等待访问者连接
				log.Printf("新连接 [%d] [%s]\n", local.Port2, local.String())
				resp := requestConnection(conn, cfg.Key, local.Port2)

				// 处理连接结果
				switch resp.Result {
				case protocolResultSuccess:
					ch <- conn
				case protocolResultVersionMismatch:
					// 版本不匹配，退出客户端
					// 鉴权失败，退出客户端
					log.Fatalln("版本不匹配")
				case protocolResultFailToAuth:
					// 鉴权失败，退出客户端
					log.Fatalln("认证失败")
				case protocolResultIllegalAccessPort:
					// 访问端口不合法
					log.Fatalln("端口不合法")
				default:
					// 连接中断，重新连接
					log.Printf("连接中断，重新连接， [%d] [%s]\n", resp.Result, local.String())
					closeConn(conn)
					flagCh <- true
				}
			}(connChan)
		}
	}
}

// 本地服务连接拨号，并建立双向通道
func buildLocalConnection(local config.NetAddress, connCh chan net.Conn, flagCh chan bool) {
	for {
		select {
		case cn := <-connCh:
			// 建立本地连接访问
			go func(conn net.Conn) {
				// 本地连接，不需要重新拨号
				if localConn := dial(local, 0); localConn != nil {
					// 通知创建新桥
					flagCh <- true
					forward(localConn, conn)
				} else {
					// 放弃连接，重新建桥
					closeConn(conn)
					flagCh <- true
				}
			}(cn)
		}
	}
}

// 入口
func Client(cfg config.ClientConfig) {
	log.Println("加载配置", cfg)

	// 遍历所有端口
	for index := range cfg.LocalAddr {
		go handleClientConnection(cfg, index)
	}
	select {}
}
