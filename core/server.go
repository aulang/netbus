package core

import (
	"github.com/aulang/netbus/config"
	"log"
	"net"
	"sync"
)

// 隧道上下文
type TunnelContext struct {
	request  Protocol      // 请求信息
	connChan chan net.Conn // 连接池
}

// key:   accessPort
// value: TunnelContext
var (
	tunnelContextMap   sync.Map
	tunnelContextMutex sync.Mutex
)

// 处理连接
func handleBridgeConnection(bridgeConn net.Conn, cfg config.ServerConfig, tunnelContextChan chan TunnelContext) {
	// 接收协议消息
	req := receiveProtocol(bridgeConn)
	// 检查请求合法性
	if protocolResult := checkRequest(req, cfg); protocolResult != protocolResultSuccess {
		log.Printf("不合法的请求, code = %b, ip = %s\n", protocolResult, bridgeConn.RemoteAddr().String())
		sendProtocol(bridgeConn, req.NewResult(protocolResult))
		closeConn(bridgeConn)
		return
	}

	// 建立连接
	context, exists := tunnelContextMap.Load(req.Port)
	if exists {
		context.(TunnelContext).connChan <- bridgeConn
		return
	}

	// 第一次创建才会执行，避免每次都加锁
	tunnelContextMutex.Lock()
	defer tunnelContextMutex.Unlock()
	context, exists = tunnelContextMap.Load(req.Port)
	if !exists {
		context = TunnelContext{
			request:  req,
			connChan: make(chan net.Conn),
		}
		tunnelContextMap.Store(req.Port, context)
		tunnelContextChan <- context.(TunnelContext)
	}
	context.(TunnelContext).connChan <- bridgeConn
}

// 检查请求信息，返回结果
func checkRequest(req Protocol, cfg config.ServerConfig) byte {
	if !req.Success() {
		return req.Result
	}
	// 检查版本号
	if req.Version != protocolVersion {
		log.Println("版本号不匹配", req.String())
		return protocolResultVersionMismatch
	}
	// 检查权限
	if _, ok := config.CheckKey(cfg.Key, req.Key); !ok {
		log.Println("认证失败", req.String())
		return protocolResultFailToAuth
	}
	// 检查访问端口是否在允许范围内
	if ok := cfg.PortInRange(req.Port); !ok {
		log.Println("访问端口不合法", req.String())
		return protocolResultIllegalAccessPort
	}
	return protocolResultSuccess
}

// 处理访问连接
func handleServerConnection(context TunnelContext) {
	listener := listen(context.request.Port)
	if listener == nil {
		tunnelContextMap.Delete(context.request.Port)
		return
	}
	for {
		serverConn := accept(listener)
		bridgeConn := <-context.connChan
		if sendProtocol(bridgeConn, context.request.NewResult(protocolResultSuccess)) {
			log.Printf("接受连接 [%d] [%s]\n", context.request.Port, serverConn.RemoteAddr().String())
			go forward(bridgeConn, serverConn)
		} else {
			log.Printf("通道中断，关闭监听器 [%d]\n", context.request.Port)
			closeConn(serverConn, bridgeConn)
			_ = listener.Close()
			tunnelContextMap.Delete(context.request.Port)
			break
		}
	}
}

// 入口
func Server(cfg config.ServerConfig) {
	log.Println("加载配置", cfg)

	// 监听桥接端口
	bridgeListener := listen(cfg.Port)
	if bridgeListener == nil {
		log.Fatalln("监听端口失败")
	}

	tunnelContextChan := make(chan TunnelContext)
	go func() {
		for {
			// 受理来自客户端的请求
			bridgeConn := accept(bridgeListener)
			if bridgeConn != nil {
				go handleBridgeConnection(bridgeConn, cfg, tunnelContextChan)
			}
		}
	}()
	// 处理监听及访问
	go func() {
		for {
			select {
			case context := <-tunnelContextChan:
				go handleServerConnection(context)
			}
		}
	}()

	select {}
}
