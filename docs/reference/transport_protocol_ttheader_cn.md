# 传输协议 TTheader

参考 [Thrift THeader 协议](https://github.com/apache/thrift/blob/master/doc/specs/HeaderFormat.md) ，我们设计了 TTheader 协议。

## 协议编码

```
 0 1 2 3 4 5 6 7 8 9 a b c d e f 0 1 2 3 4 5 6 7 8 9 a b c d e f
+----------------------------------------------------------------+
| 0|                          LENGTH                             |
+----------------------------------------------------------------+
| 0|       HEADER MAGIC          |            FLAGS              |
+----------------------------------------------------------------+
|                         SEQUENCE NUMBER                        |
+----------------------------------------------------------------+
| 0|     HEADER SIZE        | ...
+---------------------------------

                  Header is of variable size:

                   (and starts at offset 14)

+----------------------------------------------------------------+
| PROTOCOL ID  |NUM TRANSFORMS . |TRANSFORM 0 ID (uint8)| 
+----------------------------------------------------------------+
|  TRANSFORM 0 DATA ...
+----------------------------------------------------------------+
|         ...                              ...                   |
+----------------------------------------------------------------+
| INFO 0 ID (uint8)|       INFO 0  DATA ...
+----------------------------------------------------------------+
|         ...                              ...                   |
+----------------------------------------------------------------+
|                                                                |
|                              PAYLOAD                           |
|                                                                |
+----------------------------------------------------------------+
```

其中：

1. `LENGTH` 字段 32bits，包括数据包剩余部分的字节大小，不包含 `LENGTH` 自身长度
2. `HEADER MAGIC` 字段 16bits，值为：0x1000，用于标识 TTHeaderTransport
3. `FLAGS` 字段 16bits，为预留字段，暂未使用，默认值为 `0x0000`
4. `SEQUENCE NUMBER` 字段 32bits，表示数据包的 seqId，可用于多路复用，最好确保单个连接内递增
5. `HEADER SIZE` 字段 16bits，等于头部长度字节数 /4，头部长度计算从第 14 个字节开始计算，一直到 `PAYLOAD` 前（备注：header 的最大长度为 64K）
6. `PROTOCOL ID` 字段 uint8 编码，取值有：
    - ProtocolIDBinary = 0
    - ProtocolIDCompact  = 2
7. `NUM TRANSFORMS` 字段 uint8 编码，表示 `TRANSFORM` 个数
8. `TRANSFORM ID` 字段 uint8 编码，具体取值参考下文
9. `INFO ID` 字段 uint8 编码，具体取值参考下文
10. `PAYLOAD` 消息内容

## PADDING 填充

Header 部分长度 bytes 数必须是 4 的倍数，不足部分用 `0x00` 填充

## Transform IDs

表示压缩方式，为预留字段，暂不支持，取值有：

- ZLIB_TRANSFORM = 0x01，对应的 data 为空，表示用 `zlib`  压缩数据；
- SNAPPY_TRANSFORM = 0x03，对应的 data 为空，表示用 `snappy` 压缩数据；

## Info IDs

用于传递一些 kv 对信息，取值有：

- INFO_KEYVALUE = 0x01，对应的 data 为 key/value 对，key 和 value 各自都是由 uint16 的长度加上 no-trailing-null 的字符串组成，一般用于传递一些常见的 meta 信息，例如 tracingId；
- INFO_INTKEYVALUE = 0x10，对应的 data 为 key/value 对，key 为 uint16，value 由 uint16 的长度加上 no-trailing-null 的字符串组成，一般用于传递一些内部定制的 meta 信息，其中作为 request 有些 key 是必填的：
  - TRANSPORT_TYPE = 1（取值：framed/unframed）
  - LOG_ID = 2
  - FROM_SERVICE = 3
  - FROM_CLUSTER = 4（默认 default）
  - FROM_IDC = 5
  - TO_SERVICE = 6
  - TO_METHOD = 9
- ACL_TOKEN_KEYVALUE = 0x11，对应的 data 为 key/value 对，key 和 value 各自都是由 uint16 的长度加上 no-trailing-null 的字符串组成，用于传递 ACL Token；