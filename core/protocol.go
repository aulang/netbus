package core

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"time"
)

const (
	// 协议-结果
	protocolResultFail              = 0 // 失败，默认值
	protocolResultSuccess           = 1 // 成功
	protocolResultFailToSend        = 2 // 发送失败
	protocolResultFailToReceive     = 3 // 接收失败
	protocolResultFailToAuth        = 4 // 鉴权失败
	protocolResultVersionMismatch   = 5 // 版本不匹配
	protocolResultIllegalAccessPort = 6 // 访问端口不合法

	// 协议发送超时时间
	protocolSendTimeout = 3

	// 版本号(单调递增)
	protocolVersion = 4
)

// 协议格式
// 结果|版本号|访问端口|Key
// 1|2|13306|Aulang

// 协议
type Protocol struct {
	Result  byte   // 结果：0 失败，1 成功
	Version uint32 // 版本号，单调递增
	Port    uint32 // 访问端口
	Key     string // 身份验证
}

// 转字符串
func (p *Protocol) String() string {
	return fmt.Sprintf("%d|%d|%d|%s", p.Result, p.Version, p.Port, p.Key)
}

// 返回一个新结果
func (p *Protocol) NewResult(newResult byte) Protocol {
	return Protocol{
		Result:  newResult,
		Version: p.Version,
		Port:    p.Port,
		Key:     p.Key,
	}
}

func (p *Protocol) Bytes() []byte {
	buffer := bytes.NewBuffer([]byte{})

	buffer.WriteByte(p.Result)
	_ = binary.Write(buffer, binary.BigEndian, p.Version)
	_ = binary.Write(buffer, binary.BigEndian, p.Port)
	buffer.WriteString(p.Key)
	return buffer.Bytes()
}

func (p *Protocol) Len() byte {
	return byte(len(p.Bytes()))
}

// 是否成功
func (p *Protocol) Success() bool {
	return p.Result == protocolResultSuccess
}

// 解析协议
func parseProtocol(body []byte) Protocol {
	// 检查 body 长度，是否合法
	if len(body) < 10 {
		return Protocol{Result: protocolResultFail}
	}
	return Protocol{
		Result:  body[0],
		Version: binary.BigEndian.Uint32(body[1:5]),
		Port:    binary.BigEndian.Uint32(body[5:9]),
		Key:     string(body[9:]),
	}
}

// 发送协议
// 第一个字节为协议长度
// 协议长度只支持到255
func sendProtocol(conn net.Conn, req Protocol) bool {
	buffer := bytes.NewBuffer([]byte{})
	buffer.WriteByte(req.Len())
	buffer.Write(req.Bytes())

	// 设置写超时时间，避免连接断开的问题
	if err := conn.SetWriteDeadline(time.Now().Add(protocolSendTimeout * time.Second)); err != nil {
		log.Println("设置超时时间失败。", err.Error())
		return false
	}
	if _, err := conn.Write(buffer.Bytes()); err != nil {
		log.Printf("发送协议数据失败。 [%s] %s\n", req.String(), err.Error())
		return false
	}
	// 清空写超时设置
	if err := conn.SetWriteDeadline(time.Time{}); err != nil {
		log.Println("清除超时时间失败。", err.Error())
		return false
	}
	return true
}

// 接收协议
// 第一个字节为协议长度
func receiveProtocol(conn net.Conn) Protocol {
	var err error
	var length byte

	if err = binary.Read(conn, binary.BigEndian, &length); err != nil {
		return Protocol{Result: protocolResultFailToReceive}
	}
	// 读取协议内容
	body := make([]byte, length)
	if err = binary.Read(conn, binary.BigEndian, &body); err != nil {
		return Protocol{Result: protocolResultFailToReceive}
	}
	return parseProtocol(body)
}
