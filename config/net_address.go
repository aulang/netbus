package config

import (
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
)

// 网络地址
type NetAddress struct {
	IP    string
	Port  uint32
	Port2 uint32
}

// 转字符串
func (t *NetAddress) String() string {
	return fmt.Sprintf("%s:%d", t.IP, t.Port)
}

// 完整字符串
func (t *NetAddress) FullString() string {
	return fmt.Sprintf("%s:%d:%d", t.IP, t.Port, t.Port2)
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
		log.Println("Fail to parse address")
		return NetAddress{}, false
	}
	// 解析IP
	ip := strings.TrimSpace(arr[0])
	ipPattern := `^(\d{1,2}|1\d\d|2[0-4]\d|25[0-5])\.(\d{1,2}|1\d\d|2[0-4]\d|25[0-5])\.(\d{1,2}|1\d\d|2[0-4]\d|25[0-5])\.(\d{1,2}|1\d\d|2[0-4]\d|25[0-5])$`
	if ok, err := regexp.MatchString(ipPattern, ip); !ok || err != nil {
		log.Println("Fail to parse address ip")
		return NetAddress{}, false
	}
	// 解析port
	port, err := parsePort(arr[1])
	if err != nil || !checkPort(port) {
		log.Println("Fail to parse address port")
		return NetAddress{}, false
	}
	port2 := port
	// 如果配置有 port2 ，则增加解析
	if len(arr) == 3 {
		port2, err = parsePort(arr[2])
		if err != nil || !checkPort(port2) {
			log.Println("Fail to parse address port")
			return NetAddress{}, false
		}
	}
	return NetAddress{ip, uint32(port), uint32(port2)}, true
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
