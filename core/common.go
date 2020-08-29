package core

import (
	"fmt"
	"github.com/aulang/netbus/config"
	"io"
	"log"
	"net"
	"sync"
	"time"
)

const (
	// 重连间隔时间
	retryIntervalTime = 5
)

var bufferPool *sync.Pool

func init() {
	bufferPool = &sync.Pool{}
	bufferPool.New = func() interface{} {
		return make([]byte, 32*1024)
	}
}

// 使用 bufferPool 重写 copy 函数， 避免反复 gc，提升性能
func ioCopy(dst io.Writer, src io.Reader) (written int64, err error) {
	if wt, ok := src.(io.WriterTo); ok {
		return wt.WriteTo(dst)
	}
	if rt, ok := dst.(io.ReaderFrom); ok {
		return rt.ReadFrom(src)
	}

	buf := bufferPool.Get().([]byte)
	defer bufferPool.Put(buf)
	for {
		nr, er := src.Read(buf)
		if nr > 0 {
			nw, ew := dst.Write(buf[0:nr])
			if nw > 0 {
				written += int64(nw)
			}
			if ew != nil {
				err = ew
				break
			}
			if nr != nw {
				err = io.ErrShortWrite
				break
			}
		}
		if er == io.EOF {
			break
		}
		if er != nil {
			err = er
			break
		}
	}
	return written, err
}

// 连接数据复制
func connCopy(dst, src net.Conn, wg *sync.WaitGroup) {
	if _, err := ioCopy(dst, src); err != nil {
		log.Println("连接中断", err)
	}
	_ = dst.Close()
	wg.Done()
}

// 连接转发
func forward(conn1, conn2 net.Conn) {
	var wg sync.WaitGroup
	wg.Add(2)
	go connCopy(conn1, conn2, &wg)
	go connCopy(conn2, conn1, &wg)
	wg.Wait()
}

// 关闭连接
func closeConn(connections ...net.Conn) {
	for _, conn := range connections {
		if conn != nil {
			_ = conn.Close()
		}
	}
}

// 拨号
func dial(targetAddr config.NetAddress /*目标地址*/, maxRedialTimes int /*最大重拨次数*/) net.Conn {
	redialTimes := 0
	for {
		conn, err := net.Dial("tcp", targetAddr.String())
		if err == nil {
			//log.Printf("Dial to [%s] success.\n", targetAddr)
			return conn
		}
		redialTimes++
		if maxRedialTimes < 0 || redialTimes < maxRedialTimes {
			// 重连模式，每5秒一次
			log.Printf("连接到 [%s] 失败, %d秒杀之后重连(%d)。", targetAddr.String(), retryIntervalTime, redialTimes)
			time.Sleep(retryIntervalTime * time.Second)
		} else {
			log.Printf("连接到 [%s] 失败。 %s\n", targetAddr.String(), err.Error())
			return nil
		}
	}
}

// 监听端口
func listen(port uint32) net.Listener {
	address := fmt.Sprintf("0.0.0.0:%d", port)
	listener, err := net.Listen("tcp", address)
	if err != nil {
		log.Println("监听端口失败，端口已被占用", port)
		return nil
	}
	log.Println("正在监听端口", address)
	return listener
}

// 受理请求
func accept(listener net.Listener) net.Conn {
	conn, err := listener.Accept()
	if err != nil {
		log.Println("接受连接失败", err.Error())
		return nil
	}
	return conn
}
