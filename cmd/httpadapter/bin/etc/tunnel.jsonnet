local K=1024;
local Nanosecond   = 1;
local Microsecond          = 1000 * Nanosecond;
local Millisecond          = 1000 * Microsecond;
local Second               = 1000 * Millisecond;
local Minute               = 60 * Second;
local Hour                 = 60 * Minute;
// 設置默認隧道
local Options = {
    // httpadapter 服務器地址
    Server: "127.0.0.1:8000",

    // 服務器窗口大小
    Window: 32 * K,
    // 讀取緩衝區大小
    ReadBuffer: 10*K,
    // 寫入緩衝區大小
    WriteBuffer: 10*K,

    // 一段時間內沒有數據流動就發送 ping 驗證 tcp 連接是否還有效
    Ping: Second * 50,
};
[
    Options{
        // 監聽端口
        Listen: ":8964",
        // 隧道連接地址
        Backend: "tcp://ssh.king011.com:10000",
    },
    Options{
        // 監聽端口
        Listen: ":6489",
        // 隧道連接地址
        Backend: "tcp://ssh.king011.com:10002",
    },
    Options{
        // 監聽端口
        Listen: ":9444",
        // 隧道連接地址
        Backend: "tcp://ssh.king011.com:9444",
    },
]