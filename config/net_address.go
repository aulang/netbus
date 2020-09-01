package config

import (
	"fmt"
	"log"
	"strconv"
	"strings"
)

// 网络地址
type NetAddress struct {
	Host  string
	Port  uint32
	Port2 uint32
}

// 转字符串
func (n *NetAddress) String() string {
	return fmt.Sprintf("%s:%d", n.Host, n.Port)
}

// 完整字符串
func (n *NetAddress) FullString() string {
	return fmt.Sprintf("%s:%d:%d", n.Host, n.Port, n.Port2)
}

// 解析多个地址
func ParseNetAddresses(addresses string) ([]NetAddress, bool) {
	arr := strings.Split(addresses, ",")
	result := make([]NetAddress, len(arr))

	var ok bool
	for i, addr := range arr {
		if result[i], ok = ParseNetAddress(addr); !ok {
			return nil, false
		}
	}
	return result, true
}

// 解析单个网络地址
// 支持两个端口的解析，格式如192.168.1.100:3389:13389
func ParseNetAddress(address string) (NetAddress, bool) {
	arr := strings.Split(strings.TrimSpace(address), ":")
	if len(arr) < 2 {
		log.Println("解析地址失败！")
		return NetAddress{}, false
	}
	// 解析IP
	host := strings.TrimSpace(arr[0])
	if host == "" {
		log.Println("地址格式不对！")
		return NetAddress{}, false
	}
	// 解析port
	port, err := parsePort(arr[1])
	if err != nil || !checkPort(port) {
		log.Println("端口号格式不对！")
		return NetAddress{}, false
	}
	port2 := port
	// 如果配置有 port2 ，则增加解析
	if len(arr) == 3 {
		port2, err = parsePort(arr[2])
		if err != nil || !checkPort(port2) {
			log.Println("地址:端口号格式不对")
			return NetAddress{}, false
		}
	}
	return NetAddress{host, port, port2}, true
}

// 解析单个端口
func parsePort(str string) (uint32, error) {
	var port int
	var err error
	str = strings.TrimSpace(str)
	if port, err = strconv.Atoi(str); err == nil {
		return uint32(port), nil
	}
	return 0, err
}

// 检查端口是否合法
func checkPort(port uint32) bool {
	return port > 0 && port <= 65535
}
