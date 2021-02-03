package core

import (
	"github.com/aulang/netbus/config"
	"log"
	"net"
	"sync"
)

// 客户端通道
type ClientTunnel struct {
	protocol Protocol      // 请求信息
	connChan chan net.Conn // 会话连接池
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
func handleClientConn(conn net.Conn, cfg config.ServerConfig, clientTunnelChan chan ClientTunnel) {
	// 接收客户端发送的协议消息
	protocol := receiveProtocol(conn)
	// 检查请求合法性
	if protocolResult := checkProtocol(protocol, cfg); protocolResult != protocolResultSuccess {
		// 协议不合法，发送失败信息，不在处理
		sendProtocol(conn, protocol.NewResult(protocolResult))
		closeWithoutError(conn)
		return
	}

	// 发送认证成功信息
	if !sendProtocol(conn, protocol.NewResult(protocolResultSuccess)) {
		log.Println("发送认证成功信息失败！", protocol.String())
		closeWithoutError(conn)
		return
	}

	// 建立连接关系，{服务器监听端口 <-> 客户端会话连接池}
	clientTunnel, exists := clientTunnelMap.Load(protocol.Port)
	if exists {
		clientTunnel.(ClientTunnel).connChan <- conn
		return
	}

	// 第一次创建才会执行，避免每次都加锁
	clientTunnelMutex.Lock()
	defer clientTunnelMutex.Unlock()

	clientTunnel, exists = clientTunnelMap.Load(protocol.Port)

	if !exists {
		clientTunnel = ClientTunnel{
			protocol: protocol,
			connChan: make(chan net.Conn),
		}
		clientTunnelMap.Store(protocol.Port, clientTunnel)
		clientTunnelChan <- clientTunnel.(ClientTunnel)
	}

	clientTunnel.(ClientTunnel).connChan <- conn
}

// 处理端口转发，转发访问数据
func handleProxyConn(clientTunnel ClientTunnel) {
	// 代理端口号
	listenPort := clientTunnel.protocol.Port
	// 监听服务端代理端口
	listener, err := listen(listenPort)
	if err != nil {
		log.Printf("监听代理端口失败：[%d]，端口已被占用：[%s]\n", listenPort, err.Error())
		// 关闭连接
		closeWithoutError(<-clientTunnel.connChan)
		// 清除Map
		clientTunnelMap.Delete(listenPort)
		return
	}
	log.Printf("正在监听指定代理端口：[%d]\n", listenPort)

	for {
		proxyConn, err := listener.Accept()
		if err != nil {
			log.Println("接受代理端口连接失败！", err)
			closeWithoutError(proxyConn)
			continue
		}

		for clientConn := range clientTunnel.connChan {
			// 进行数据转发
			go forward(proxyConn, clientConn)
		}
	}
}

// 入口
func Server(cfg config.ServerConfig) {
	log.Println("加载服务端配置：", cfg)

	// 服务器监听端口 <-> 客户端会话池
	clientTunnelChan := make(chan ClientTunnel)

	// 受理来自客户端连接请求
	go func(cfg config.ServerConfig, clientTunnelChan chan ClientTunnel) {
		// 监听桥接端口
		listener, err := listen(cfg.Port)
		if err != nil {
			log.Fatalf("监听端口失败：[%d]，端口已被占用：[%s]\n", cfg.Port, err.Error())
		}

		for {
			conn, err := listener.Accept()
			if err != nil {
				log.Println("接受客户端会话失败！", err)
				continue
			}
			go handleClientConn(conn, cfg, clientTunnelChan)
		}
	}(cfg, clientTunnelChan)

	// 处理代理连接，转发访问数据
	for clientTunnel := range clientTunnelChan {
		go handleProxyConn(clientTunnel)
	}
}
