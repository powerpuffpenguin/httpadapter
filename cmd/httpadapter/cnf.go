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
	Rewriter []Rewriter
	// 對這些 Host 值使用 h2c 連接上游服務
	H2C H2C
}
type H2C struct {
	// Host 匹配規則，不設置則不進行匹配
	// 詳細匹配規則和Schemes相同
	Hosts []string
	// Hostname 匹配規則，不設置則不進行匹配
	// 詳細匹配規則和Schemes相同
	Hostnames []string
}
type Rewriter struct {
	// Scheme 匹配規則，不設置則匹配任意 Scheme
	// 數組中任意一個匹配則任務匹配重寫條件
	// * 'abc.com' 字符串完全匹配
	// * '*' * 標記匹配任意內容
	// * '^http' ^ 標記匹配字符串前綴
	// * 's$' $ 標記匹配字符串後綴
	// * '@(https)|(wss)' @ 標記使用正在表達式進行匹配
	Schemes []string
	// Host 匹配規則，不設置則匹配任意 Host
	// 詳細匹配規則和Schemes相同
	Hosts []string
	// Hostname 匹配規則，不設置則匹配任意 Hostname
	// 詳細匹配規則和Schemes相同
	Hostnames []string
	// 如果爲 true 則拒絕對此地址的轉發
	Reject bool

	// 要 修改的 Scheme 如果不設置則 不改變
	// tcp/tls/ws/wss/http/https
	Scheme string
	// 要 修改的 Host 如果不設置則 不改變
	Host string
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
	// 使用 tls 連接服務器
	TLS bool
	// 不用 驗證服務器證書
	Insecure bool
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
