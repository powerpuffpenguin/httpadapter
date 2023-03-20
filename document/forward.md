# 中轉層

每個 http 請求都需要放到一個單獨的 channel 中進行請求，安裝請求類型的不同可分爲:

* [一元請求](#一元請求)
* [流式請求](#流式請求)

# Message

通常，客戶端需要先發送一個 Message 給服務器，其中包含了要請求的 http 信息，服務器會對齊進行代理訪問並且將結果以一個 Message 進行返回。 Message 定義如下

| 字段 | 偏移 | 字節 | 含義 |
|--- |--- |---|---|
|   metalen  |   0   |  2   |   元信息大小    |
|   bodylen  |   2   |  8   |   body 大小    |
|   metadata  |   10   |  由 metalen 指定   |   一個json編碼的 http 元信息    |
|   body  |   10+metalen   |  由 bodylen 指定   |   http 請求/響應的 body    |

# 一元請求

一元請求是對大部分標準 http 請求的中轉，它首先由客戶端發送一個 Message 給服務器服務器，之後服務器將處理結果也包裝爲一個 Message 返回給客戶端


客戶端請求 metadata 定義如下：

```
{
    // 指定了要請求的 http 接口網址
    "url": "http://xxx/api/v1", 

    // 以什麼方法請求 http 接口:
    // * GET 
    // * HEAD 
    // * POST 
    // * PUT 
    // * PATCH 
    // * DELETE
    "method": "GET",
    
    // 這是可選字段，如果設置了會將它的每個子屬性添加到 http header 中
    // 每個子屬性的值，應該是一個 字符串數組
    "header": {
        // 這裏指定了添加一個 User-Agent 屬性，將客戶端僞裝爲 firefox 瀏覽器
        "User-Agent" : [ "Mozilla/5.0 (X11; Linux x86_64; rv:109.0) Gecko/20100101 Firefox/109.0" ],

        // 這裏指定添加一個 Accept 屬性，告訴 http 服務器優先返回 json 編碼的數據
        "Accept": ["application/json" , "text/plain" , "*/*"]
    }
}
```


服務器響應 metadata 定義如下:

```
{
    // http 的響應碼
    "status": 200,
    // http 服務器響應的 heaer ，大部分時候都可以直接忽略
    "header": {
        "Content-Length": ["316"],
        "Content-Type": ["application/json"],
        "Date": ["Tue, 14 Mar 2023 07:18:43 GMT"],
        "Server": ["nginx/1.18.0 (Ubuntu)"]
    },
}
```

> 注意一元請求是爲了能夠訪問服務器提供的 http api 接口，服務器必須設置 context-length 屬性，因爲如果不設置 context-length 中轉程序無法預估中轉成本也無法提前組響應包這樣必須將body全部讀取才能中轉數據，這樣的話黑客可以要求中轉程序請求一個 response.body 巨大的惡意接口從而導致服務器內存耗盡而崩潰。
> 不過好在 http 的 api 接口幾乎 99.99% 的接口都設置了 context-length，而通常只有啓用了 gzip 等自動壓縮的檔案下載才會不設置此屬性。

# 流式請求

流式請求主要用於轉發 websocket 或 tcp 流，通常首先由客戶端發送一個 Message 裏面包含了轉發信息，然後由服務器返回一個 Message 如果返回的 Message 沒有錯誤就可以進行後續流傳輸

## websocket

1. 首先由客戶端發送一個 Message 其 metadata 定義如下:

    ```
    {
        // 指定了要請求的 websocket 接口網址
        "url": "ws://xxx/api/v1", 
        
        // 這是可選字段，如果設置了會將它的每個子屬性添加到 http header 中
        // 每個子屬性的值，應該是一個 字符串數組
        "header": {
            // 這裏指定了添加一個 User-Agent 屬性，將客戶端僞裝爲 firefox 瀏覽器
            "User-Agent" : [ "Mozilla/5.0 (X11; Linux x86_64; rv:109.0) Gecko/20100101 Firefox/109.0" ],
        }
    }
    ```

    目前不允許在發送 message 時設置 body

2.  服務器會響應 Message 其 metadata 定義如下:

    ```
    {
        // 這個值必須是 101 表示切換協議成功，其它任何值都代表了錯誤
        "status": 101,
        // http 服務器響應的 heaer ，大部分時候都可以直接忽略
        "header": {
            "Content-Length": ["316"],
            "Content-Type": ["application/json"],
            "Date": ["Tue, 14 Mar 2023 07:18:43 GMT"],
            "Server": ["nginx/1.18.0 (Ubuntu)"]
        },
    }
    ```
3. 一旦步驟2成功就可以在 channel 中直接進行雙向的 Frame 傳輸

    Frame 由 2 部分組成 一個字節的 FrameHeader 和 未知字節的 FrameData 數組組成， FrameData 至少存在一個

    FrameHeader 表示了當前幀的類型其定義同 websocket 一致:
    * **1** 當前幀是文本
    * **2** 當前幀是二進制數據
    * **8** 當前幀是可選的 控制幀 close
    * **9** 當前幀是可選的 控制幀 ping
    * **10** 當前幀是可選的 控制幀 pong

    FrameData 由 DataFlag(1 字節)+DataLen(2字節)+DataBinary(二進制數據) 組成

    在 FrameData 數組中，最後一個 FrameData 的 DataFlag 需要設置爲 1 表示這一幀結束其它的 DataFlag 設置爲 0；DataLen 記錄了 DataBinary 的長度； DataBinary 是要寫入到 websocket(或從 websocket 中讀取) 的二進制數據

## tcp

1. 首先由客戶端發送一個 Message 其 metadata 定義如下:

    ```
    {
        // 指定了要請求的 websocket 接口網址，tcp 協議必須指定端口
        // 可以使用 tls 替代 tcp 則會使用 tls 加密的tcp連接後端服務
        "url": "tcp://127.0.0.1:9000", 
        "url": "tls://127.0.0.1:9000", 
    }
    ```

    目前不允許在發送 message 時設置 body

2.  服務器會響應 Message 其 metadata 定義如下:

    ```
    {
        // 這個值必須是 101 表示切換協議成功，其它任何值都代表了錯誤
        "status": 101,
    }
    ```
3. 一旦步驟2成功就可以在 channel 中直接進行雙向的數據流傳輸， channel 會原封不動的在前後端之間轉發 tcp 數據