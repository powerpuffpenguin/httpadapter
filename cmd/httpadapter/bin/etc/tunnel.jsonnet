local KB = 1024;
local MB = KB * 1024;

local Nanosecond = 1;
local Microsecond = 1000 * Nanosecond;
local Millisecond = 1000 * Microsecond;
local Second = 1000 * Millisecond;
local Minute = 60 * Second;
local Hour = 60 * Minute;
// 設置默認隧道
local Options = {
  // httpadapter 服務器地址
  Server: '127.0.0.1:8000',
  //   是否使用 tls 連接服務器
  TLS: false,
  //   如果爲 true 使用 tls 連接服務器時不驗證服務器證書
  Insecure: false,

  // 服務器窗口大小
  Window: MB * 10,
  // 讀取緩衝區大小
  ReadBuffer: MB * 10,
  // 寫入緩衝區大小
  WriteBuffer: MB * 10,

  // 一段時間內沒有數據流動就發送 ping 驗證 tcp 連接是否還有效
  Ping: Second * 50,
};
[
  Options {
    // 監聽端口
    Listen: ':8964',
    // 隧道連接地址
    Backend: 'tcp://127.0.0.1:10000',
  },
  Options {
    // 監聽端口
    Listen: ':6489',
    // 隧道連接地址
    Backend: 'tcp://127.0.0.1:10002',
  },
  Options {
    // 監聽端口
    Listen: ':9444',
    // 隧道連接地址
    Backend: 'tcp://127.0.0.1:9444',
  },
]
