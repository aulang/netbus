package test

import (
	"github.com/aulang/netbus/config"
	"github.com/aulang/netbus/core"
	"testing"
)

func TestServer(t *testing.T) {
	cfg := config.ServerConfig{
		Key:          "Aulang",
		Port:         8888,
		MinProxyPort: 10000,
		MaxProxyPort: 20000,
	}
	core.Server(cfg)
}

func TestClient(t *testing.T) {
	cfg := config.ClientConfig{
		Key: "Aulang",
		ServerAddr: config.NetAddress{
			Host: "127.0.0.1", Port: 8888,
		},
		ProxyAddrs: []config.NetAddress{
			{"127.0.0.1", 7001, 17001},
		},
		TunnelCount: 1,
	}
	core.Client(cfg)
}
