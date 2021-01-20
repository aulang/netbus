package core

import (
	"context"
	"github.com/aulang/netbus/config"
	"github.com/lucas-clemente/quic-go"
	"log"
	"net"
	"sync"
)

// 客户端通道
type ClientTunnel struct {
	protocol    Protocol          // 请求信息
	sessionChan chan quic.Session // 会话连接池
}

var (
	// key:   proxyPort
	// value: ClientTunnel
	clientTunnelMap   sync.Map
	clientTunnelMutex sync.Mutex
)

// 检查请求信息，返回结果
func checkProtocol(protocol Protocol, cfg config.ServerConfig) byte {
	// 检查版本号
	if protocol.Version != protocolVersion {
		log.Println("版本号不匹配！", protocol.String())
		return protocolResultVersionMismatch
	}
	// 检查密钥
	if _, ok := config.CheckKey(cfg.Key, protocol.Key); !ok {
		log.Println("认证失败！", protocol.String())
		return protocolResultFailToAuth
	}
	// 检查访问端口是否在允许范围内
	if ok := cfg.PortInRange(protocol.Port); !ok {
		log.Println("访问端口不合法！", protocol.String())
		return protocolResultIllegalAccessPort
	}
	return protocolResultSuccess
}

// 处理客户端请求
func handleClientConnection(clientSession quic.Session, cfg config.ServerConfig, clientTunnelChan chan ClientTunnel) {
	// 接收客户端发送的协议消息
	protocol := receiveProtocol(clientSession)
	// 检查请求合法性
	if protocolResult := checkProtocol(protocol, cfg); protocolResult != protocolResultSuccess {
		// 协议不合法，发送失败信息，不在处理
		sendProtocol(clientSession, protocol.NewResult(protocolResult))
		return
	}

	// 发送认证成功信息
	if !sendProtocol(clientSession, protocol.NewResult(protocolResultSuccess)) {
		log.Println("发送认证成功信息失败！", protocol.String())
		return
	}

	// 建立连接关系，{服务器监听端口 <-> 客户端会话连接池}
	clientTunnel, exists := clientTunnelMap.Load(protocol.Port)
	if exists {
		clientTunnel.(ClientTunnel).sessionChan <- clientSession
		return
	}

	// 第一次创建才会执行，避免每次都加锁
	clientTunnelMutex.Lock()
	defer clientTunnelMutex.Unlock()

	clientTunnel, exists = clientTunnelMap.Load(protocol.Port)

	if !exists {
		clientTunnel = ClientTunnel{
			protocol:    protocol,
			sessionChan: make(chan quic.Session),
		}
		clientTunnelMap.Store(protocol.Port, clientTunnel)
		clientTunnelChan <- clientTunnel.(ClientTunnel)
	}

	clientTunnel.(ClientTunnel).sessionChan <- clientSession
}

// 处理端口转发，转发访问数据
func handleProxyConnection(clientTunnel ClientTunnel) {
	// 代理端口号
	listenPort := clientTunnel.protocol.Port
	// 监听服务端代理端口
	listener, err := tcpListen(listenPort)
	if err != nil {
		log.Printf("监听代理端口失败：[%d]，端口已被占用：[%s]\n", listenPort, err.Error())
		clientTunnelMap.Delete(listenPort)
		return
	}
	log.Printf("正在监听指定代理端口：[%d]\n", listenPort)

	for {
		proxyConnection, err := listener.Accept()
		if err != nil {
			log.Println("接受代理端口连接失败！", err)
			continue
		}

		for clientSession := range clientTunnel.sessionChan {
			go func(proxyConnection net.Conn, clientSession quic.Session) {
				// 打开客户端连接，转发代理数据
				clientStream, err := clientSession.OpenStreamSync(context.Background())
				if err != nil {
					// 打开客户端流失败，关闭连接
					log.Println("打开客户端流失败！", err)
					closeWithoutError(proxyConnection, clientStream)
					return
				}
				// 进行数据传输
				forward(proxyConnection, clientStream, nil)
			}(proxyConnection, clientSession)
		}
	}
}

// 入口
func Server(cfg config.ServerConfig) {
	log.Println("加载服务端配置：", cfg)

	// 服务器监听端口 <-> 客户端会话池
	clientTunnelChan := make(chan ClientTunnel)

	// 受理来自客户端连接请求
	go func(tunnelContextChan chan ClientTunnel) {
		// 监听桥接端口
		listener, err := listen(cfg.Port)
		if err != nil {
			log.Fatalf("监听端口失败：[%d]，端口已被占用：[%s]\n", cfg.Port, err.Error())
		}

		for {
			session, err := listener.Accept(context.Background())
			if err != nil {
				log.Println("接受客户端会话失败！", err)
				continue
			}
			go handleClientConnection(session, cfg, clientTunnelChan)
		}
	}(clientTunnelChan)

	// 处理代理连接，转发访问数据
	for clientTunnel := range clientTunnelChan {
		go handleProxyConnection(clientTunnel)
	}
}
