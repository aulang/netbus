package core

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"net"
)

const (
	// 协议-结果
	protocolResultFail              = 0 // 失败，默认值
	protocolResultSuccess           = 1 // 成功
	protocolResultFailToReceive     = 2 // 接收失败
	protocolResultFailToAuth        = 3 // 鉴权失败
	protocolResultVersionMismatch   = 4 // 版本不匹配
	protocolResultIllegalAccessPort = 5 // 访问端口不合法

	// 版本号(单调递增)
	protocolVersion = 1
)

// 协议格式
// 结果|版本号|访问端口|Key
// 1|2|17001|Aulang

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
func sendProtocol(conn net.Conn, protocol Protocol) bool {
	pbs := protocol.Bytes()

	buffer := bytes.NewBuffer([]byte{})
	// 数据长度
	buffer.WriteByte(byte(len(pbs)))
	buffer.Write(pbs)

	// 发送协议数据
	if _, err := conn.Write(buffer.Bytes()); err != nil {
		log.Println("发送协议数据失败！", err)
		return false
	}

	return true
}

// 接收协议
// 第一个字节为协议长度
func receiveProtocol(conn net.Conn) Protocol {
	var length byte
	if err := binary.Read(conn, binary.BigEndian, &length); err != nil {
		log.Println("接受协议数据失败！", err)
		return Protocol{Result: protocolResultFailToReceive}
	}
	// 读取协议内容
	body := make([]byte, length)
	if err := binary.Read(conn, binary.BigEndian, &body); err != nil {
		log.Println("接受协议数据失败！", err)
		return Protocol{Result: protocolResultFailToReceive}
	}

	return parseProtocol(body)
}
