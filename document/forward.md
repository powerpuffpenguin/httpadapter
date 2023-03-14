# 中轉層

每個 http 請求都需要放到一個單獨的 channel 中進行請求，安裝請求類型的不同可分爲:

* [一元請求](#一元請求)
* [流式請求](#流式請求)

# Message

通常，客戶端需要先發送一個 Message 給服務器，其中包含了要請求的 http 信息，服務器會對齊進行代理訪問並且將結果以一個 Message 進行返回。 Message 定義如下

| 字段 | 偏移 | 字節 | 含義 |
|--- |--- |---|---|
|   metalen  |   0   |  2   |   元信息大小    |
|   bodylen  |   2   |  4   |   body 大小    |
|   metadata  |   6   |  由 metalen 指定   |   一個json編碼的 http 元信息    |
|   body  |   6+metalen   |  由 bodylen 指定   |   http 請求/響應的 body    |

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