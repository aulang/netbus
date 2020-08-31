package test

import (
	"github.com/aulang/netbus/config"
	"github.com/aulang/netbus/core"
	"testing"
)

func TestServer(t *testing.T) {
	cfg := config.ServerConfig{
		Key:           "Aulang",
		Port:          8888,
		MinAccessPort: 10000,
		MaxAccessPort: 20000,
	}
	core.Server(cfg)
}

func TestClient(t *testing.T) {
	cfg := config.ClientConfig{
		Key: "Aulang",
		ServerAddr: config.NetAddress{
			Host: "127.0.0.1", Port: 8888,
		},
		LocalAddr: []config.NetAddress{
			{"127.0.0.1", 7001, 17001},
		},
		TunnelCount: 2,
	}
	core.Client(cfg)
}
