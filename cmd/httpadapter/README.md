# httpadapter

這是實現了 httpadapter 協議的一個應用軟體，可以使用它來訪問 httpadapter 協議，對於所有的指令你都可以使用 -h 參數來查看具體用法

# server

server 指令默認加載 etc/server.jsonnet 設定檔案，並啓動一個 httpadapter 服務器。

如果沒有設定 Backend，則 server 會在 httpadapter 服務端口上運行一個兼容的 http 服務，這個端口除了可以正常處理 httpadapter 客戶請求還可以使用 http 協議來訪問一些調試功能，下面是這些 http 接口的路徑

* **/document** 包含了當前 httpadapter 協議的定義說明
* **/debug/pprof/** 由 net/http/pprof 包提供的 go 測試接口

# tunnel

tunnel 指令可以開啓一個隧道，它利用了 httpadapter 協議邏輯層的 tcp 轉發功能，簡單來說 tunnel 會監聽一些 tcp 本地端口，然後將連接本地端口的這些 tcp 通過 httpadapter 轉發到 httpadapter 服務器後端。

tunnel 常見的應用場景是訪問 httpadapter 服務器後的局域網，另外如果在朝鮮可以將它作爲一個代理包裝，例如使用它來轉發 xray 協議以避免朝鮮政府對 xray 協議的探測。

# echo

echo 是一個 httpadapter 模擬服務器，echo 會接收來自 httpadapter 協議的請求並返回一個模擬的響應數據，以供測試 httpadapter 協議

# client

client 是一個 httpadapter 協議的客戶端，你可以使用它來向 httpadapter 服務器發送各種請求，以測試服務器是否正常工作