package core

import (
	"context"
	"github.com/aulang/netbus/config"
	"github.com/lucas-clemente/quic-go"
	"log"
	"sync"
	"time"
)

// 隧道上下文
type TunnelContext struct {
	request     Protocol          // 请求信息
	sessionChan chan quic.Session // 会话连接池
}

// key:   accessPort
// value: TunnelContext
var (
	tunnelContextMap   sync.Map
	tunnelContextMutex sync.Mutex
)

// 处理客户端QUIC请求
func handleBridgeSession(session quic.Session, cfg config.ServerConfig, tunnelContextChan chan TunnelContext) {
	// 接收客户端发送的协议消息
	req := receiveProtocol(session)
	// 检查请求合法性
	if protocolResult := checkRequest(req, cfg); protocolResult != protocolResultSuccess {
		// 协议不合法，发送失败信息，不在处理
		sendProtocol(session, req.NewResult(protocolResult))
		return
	}

	// 建立连接关系，{服务器监听端口 <-> 客户端会话连接池}
	tc, exists := tunnelContextMap.Load(req.Port)
	if exists {
		tc.(TunnelContext).sessionChan <- session
		return
	}

	// 第一次创建才会执行，避免每次都加锁
	tunnelContextMutex.Lock()
	defer tunnelContextMutex.Unlock()
	tc, exists = tunnelContextMap.Load(req.Port)
	if !exists {
		tc = TunnelContext{
			request:     req,
			sessionChan: make(chan quic.Session),
		}
		tunnelContextMap.Store(req.Port, tc)
		tunnelContextChan <- tc.(TunnelContext)
	}
	tc.(TunnelContext).sessionChan <- session
}

// 检查请求信息，返回结果
func checkRequest(req Protocol, cfg config.ServerConfig) byte {
	if !req.Success() {
		return req.Result
	}
	// 检查版本号
	if req.Version != protocolVersion {
		log.Println("版本号不匹配！", req.String())
		return protocolResultVersionMismatch
	}
	// 检查权限
	if _, ok := config.CheckKey(cfg.Key, req.Key); !ok {
		log.Println("认证失败！", req.String())
		return protocolResultFailToAuth
	}
	// 检查访问端口是否在允许范围内
	if ok := cfg.PortInRange(req.Port); !ok {
		log.Println("访问端口不合法！", req.String())
		return protocolResultIllegalAccessPort
	}
	return protocolResultSuccess
}

// 处理端口转发，转发访问数据
func handleServerConnection(tunnelContext TunnelContext) {
	// 监听服务端端口TCP数据
	listener, err := tcpListen(tunnelContext.request.Port)
	if err != nil {
		log.Printf("监听指定端口失败：[%d]，端口已被占用：[%s]", tunnelContext.request.Port, err.Error())
		tunnelContextMap.Delete(tunnelContext.request.Port)
		return
	}
	log.Printf("正在监听指定端口：[%d]\n", tunnelContext.request.Port)

	for {
		serverConn, err := listener.Accept()
		if err != nil {
			log.Println("接受服务端口连接失败！", err)
			continue
		}

		select {
		case bridgeSession := <-tunnelContext.sessionChan:
			{
				// 发送成功协议，连接保活
				if sendProtocol(bridgeSession, tunnelContext.request.NewResult(protocolResultSuccess)) {
					// 打开流进行数据传输
					bridgeStream, err := bridgeSession.OpenStream()
					if err != nil {
						// 打开客户端流失败，关闭连接
						log.Printf("打开客户端流失败！")
						closeWithoutError(serverConn)
						continue
					}
					// 进行数据传输
					go forward(serverConn, bridgeStream)
				} else {
					// 发送协议失败，关闭连接
					log.Printf("发送客户端协议失败，关闭服务器连接！")
					closeWithoutError(serverConn)
					continue
				}
			}
		case <-time.After(protocolSendTimeout * time.Second):
			{
				// 超时未拿到客户端连接，断开连接，停止端口监听
				log.Printf("获取客户端连接超时，关闭服务器连接和该端口监听！")
				tunnelContextMap.Delete(tunnelContext.request.Port)
				closeWithoutError(serverConn)
				break
			}
		}
	}
}

// 入口
func Server(cfg config.ServerConfig) {
	log.Println("加载服务端配置：", cfg)

	// 监听客户端QUIC桥接端口
	bridgeListener, err := listen(cfg.Port)
	if err != nil {
		log.Fatalf("监听端口失败：[%d]，端口已被占用：[%s]\n", cfg.Port, err.Error())
	}

	// 服务器监听端口 <-> 客户端会话池
	tunnelContextChan := make(chan TunnelContext)

	// 受理来自客户端QUIC连接请求
	go func() {
		for {
			bridgeSession, err := bridgeListener.Accept(context.Background())
			if err != nil {
				log.Println("接受客户端会话失败！", err)
				continue
			}
			go handleBridgeSession(bridgeSession, cfg, tunnelContextChan)
		}
	}()

	// 处理端口转发，转发访问数据
	go func() {
		for tunnelContext := range tunnelContextChan {
			go handleServerConnection(tunnelContext)
		}
	}()

	select {}
}
