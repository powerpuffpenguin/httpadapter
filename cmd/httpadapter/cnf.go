package main

import (
	"encoding/json"
	"time"

	"github.com/google/go-jsonnet"
)

type Server struct {
	// 監聽端口
	Listen string
	// tls 證書，如果設置將監聽 tls
	CertFile string
	KeyFile  string
	Options  struct {
		// 後端服務地址
		Backend string

		// 服務器窗口大小
		Window uint32
		// 服務器等待連接超時時間
		Timeout time.Duration
		// 讀取緩衝區大小
		ReadBuffer int
		// 寫入緩衝區大小
		WriteBuffer int
		// 允許併發的通道數量，<1 則不限制
		Channels int
		// 一段時間內沒有數據流動就發送 ping 驗證 tcp 連接是否還有效
		Ping time.Duration
	}
}

func loadObject(filename string, obj any) (e error) {
	vm := jsonnet.MakeVM()
	str, e := vm.EvaluateFile(filename)
	if e != nil {
		return
	}
	e = json.Unmarshal([]byte(str), obj)
	return
}
func LoadServer(filename string) (cnf *Server, e error) {
	var tmp Server
	e = loadObject(filename, &tmp)
	if e != nil {
		return
	}
	cnf = &tmp
	return
}

type Tunnel struct {
	// 監聽端口
	Listen string
	// httpadapter 服務器地址
	Server string
	// 隧道連接地址
	Backend string

	// 服務器窗口大小
	Window uint32
	// 讀取緩衝區大小
	ReadBuffer int
	// 寫入緩衝區大小
	WriteBuffer int

	// 一段時間內沒有數據流動就發送 ping 驗證 tcp 連接是否還有效
	Ping time.Duration
}

func LoadTunnel(filename string) (cnf []Tunnel, e error) {
	var tmp []Tunnel
	e = loadObject(filename, &tmp)
	if e != nil {
		return
	}
	cnf = tmp
	return
}
