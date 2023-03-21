local KB=1024;
local MB=KB*1024;

local Nanosecond   = 1;
local Microsecond          = 1000 * Nanosecond;
local Millisecond          = 1000 * Microsecond;
local Second               = 1000 * Millisecond;
local Minute               = 60 * Second;
local Hour                 = 60 * Minute;
{
    // 監聽端口
    Listen: ":8000",
    	// tls 證書，如果設置將監聽 tls
	CertFile: "",
	KeyFile: "",
    Options: {
        // 後端服務地址
        // Backend: "127.0.0.1:80",

        // 服務器窗口大小
        Window: MB * 2,
        // 服務器等待連接超時時間
        Timeout: Second * 5,
        // 讀取緩衝區大小
        ReadBuffer: MB * 2,
        // 寫入緩衝區大小
        WriteBuffer: MB * 2,
        // 允許併發的通道數量，<1 則不限制
        // Channels: 10000 ,
        // 一段時間內沒有數據流動就發送 ping 驗證 tcp 連接是否還有效
    	// Ping: Second * 50,
    },
}