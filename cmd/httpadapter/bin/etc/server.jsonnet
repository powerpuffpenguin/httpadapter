local KB = 1024;
local MB = KB * 1024;

local Nanosecond = 1;
local Microsecond = 1000 * Nanosecond;
local Millisecond = 1000 * Microsecond;
local Second = 1000 * Millisecond;
local Minute = 60 * Second;
local Hour = 60 * Minute;
{
  // 監聽端口
  Listen: ':8000',
  // tls 證書，如果設置將監聽 tls
  CertFile: '',
  KeyFile: '',
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
  // 對這些請求，重寫請求目標，也可以限制轉發目標以避免可以任何訪問服務器區域網路引起的安全問題
  // Rewriter 安裝先後順序進行匹配，一旦一個 Rewriter 被匹配就不會再處理後續 Rewriter
  Rewriter: [
    {
      Schemes: [
        'tcp',
        'tls',
      ],
      Reject: true,  // 禁止轉發 tcp/tls
    },
    // {
    //   Schemes: [
    //     'http',
    //   ],
    //   Hosts: [
    //     'www.google.com',
    //   ],
    //   Scheme: 'https',  // 將 http 協議轉爲 https
    //   Host: 'www.bing.com',  // 將對 google 的請求轉爲對 bing 的請求
    // },
  ],
  // 對匹配的上游使用 h2c 連接
  H2C: {
    // Hosts: [
    //   '*',
    //   'dbf$',
    //   '@abc',
    //   'abc',
    // ],
    // Hostnames: ['^127.0.0.1'],
  },
}
