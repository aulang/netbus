package core

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"github.com/aulang/netbus/config"
	"github.com/lucas-clemente/quic-go"
	"io"
	"log"
	"math/big"
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

func newConfig() *quic.Config {
	return &quic.Config{
		MaxIdleTimeout: time.Minute,
		KeepAlive:      true,
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

// TCP拨号
func tcpDial(targetAddr config.NetAddress /*目标地址*/, maxRedialTimes int /*最大重拨次数*/) net.Conn {
	redialTimes := 0
	for {
		conn, err := net.Dial("tcp", targetAddr.String())
		if err == nil {
			return conn
		}
		redialTimes++
		if maxRedialTimes < 0 || redialTimes < maxRedialTimes {
			// 重连模式，每5秒一次
			log.Printf("连接到 [%s] 失败, %d秒之后重连(%d)。", targetAddr.String(), retryIntervalTime, redialTimes)
			time.Sleep(retryIntervalTime * time.Second)
		} else {
			log.Printf("连接到 [%s] 失败。 %s\n", targetAddr.String(), err.Error())
			return nil
		}
	}
}

// TCP监听端口
func tcpListen(port uint32) (net.Listener, error) {
	address := fmt.Sprintf("0.0.0.0:%d", port)
	return net.Listen("tcp", address)
}

// 生成公钥和密钥
func generateTLSConfig() *tls.Config {
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		panic(err)
	}
	template := x509.Certificate{SerialNumber: big.NewInt(1)}
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		panic(err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		panic(err)
	}
	return &tls.Config{
		Certificates: []tls.Certificate{tlsCert},
		NextProtos:   []string{"quic"},
	}
}

// 拨号
func dial(targetAddr config.NetAddress /*目标地址*/, maxRedialTimes int /*最大重拨次数*/) quic.Session {
	redialTimes := 0
	tlsConf := &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{"quic"},
	}

	for {
		session, err := quic.DialAddr(targetAddr.String(), tlsConf, newConfig())
		if err == nil {
			return session
		}

		redialTimes++

		if maxRedialTimes < 0 || redialTimes < maxRedialTimes {
			// 重连模式，每5秒一次
			log.Printf("连接到 [%s] 失败, %d秒之后重连(%d)。", targetAddr.String(), retryIntervalTime, redialTimes)
			time.Sleep(retryIntervalTime * time.Second)
		} else {
			log.Printf("连接到 [%s] 失败。 %s\n", targetAddr.String(), err.Error())
			return nil
		}
	}
}

// 监听端口
func listen(port uint32) (quic.Listener, error) {
	address := fmt.Sprintf("0.0.0.0:%d", port)
	return quic.ListenAddr(address, generateTLSConfig(), newConfig())
}

// 连接数据复制
func quicCopy(src io.ReadCloser, dst io.WriteCloser, wg *sync.WaitGroup) {
	if _, err := ioCopy(dst, src); err != nil {
		log.Println("连接中断！", err)
	}
	wg.Done()
}

// 连接转发
func forward(src io.ReadWriteCloser, dst io.ReadWriteCloser) {
	var wg sync.WaitGroup

	wg.Add(2)
	go quicCopy(src, dst, &wg)
	go quicCopy(dst, src, &wg)
	wg.Wait()

	closeWithoutError(src, dst)
}
