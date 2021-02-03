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

// 关闭连接
func closeWithoutError(closers ...io.Closer) {
	for _, closer := range closers {
		if closer != nil {
			_ = closer.Close()
		}
	}
}

// 拨号
func dial(targetAddr config.NetAddress /*目标地址*/, maxRedialTimes int /*最大重拨次数*/) net.Conn {
	redialTimes := 0
	for {
		conn, err := net.Dial("tcp", targetAddr.String())
		if err == nil {
			return conn
		}
		redialTimes++
		if maxRedialTimes < 0 || redialTimes < maxRedialTimes {
			// 重连模式，每5秒一次
			log.Printf("连接到 [%s] 失败, %d秒之后重连(%d)。\n", targetAddr.String(), retryIntervalTime, redialTimes)
			time.Sleep(retryIntervalTime * time.Second)
		} else {
			log.Printf("连接到 [%s] 失败。 %s\n", targetAddr.String(), err.Error())
			return nil
		}
	}
}

// TCP监听端口
func listen(port uint32) (net.Listener, error) {
	address := fmt.Sprintf("0.0.0.0:%d", port)
	return net.Listen("tcp", address)
}

// 连接数据复制
func netCopy(src io.ReadCloser, dst io.WriteCloser, wg *sync.WaitGroup) {
	wg.Add(1)

	if _, err := ioCopy(dst, src); err != nil {
		log.Println("连接中断！", err)
	}

	closeWithoutError(dst)

	wg.Done()
}

// 连接数据转发
func forward(src io.ReadWriteCloser, dst io.ReadWriteCloser) {
	var wg sync.WaitGroup

	go netCopy(src, dst, &wg)
	go netCopy(dst, src, &wg)

	wg.Wait()
}
