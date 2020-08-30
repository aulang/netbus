package test

import (
	"fmt"
	"log"
	"net/http"
	"testing"
)

// 本地模拟启动Web服务

func handleWebRequest(w http.ResponseWriter, r *http.Request) {
	log.Println("path:", r.URL.Path)
	_, _ = fmt.Fprintf(w, r.Host)
}

func listenOnPort(port int) {
	log.Println("监听端口：", port)
	http.HandleFunc("/", handleWebRequest)
	err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
	if err != nil {
		log.Fatal("监听端口失败: ", err)
	}
}

func TestWeb7001(t *testing.T) {
	listenOnPort(7001)
}
