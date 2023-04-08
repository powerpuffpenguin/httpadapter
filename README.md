# httpadapter

httpadapter 是一個 http 適配器，它主要目的是爲 http 服務器提供一個簡單的 tcp 包裝，讓不支持 http 的平臺(例如嵌入式)可以使用簡單的 tcp 訪問 http 服務器提供的 api，這樣服務器可以得到標準 http 的所有好處而客戶端也可以使用更簡單高效的(相對 http1.1 和之前版本)方式與服務器溝通

更多詳情請參考 [httpadapter 協議](document/httpadapter.md)

# docker

run as server
```
docker run \
    -p "8000:8000" \
    -v "Your_Conf_Dir:/etc/httpadapter/etc:ro" \
    -d king011/httpadapter:1.0
```

run client
```
docker run --rm king011/httpadapter:1.0 httpadapter client -h
```