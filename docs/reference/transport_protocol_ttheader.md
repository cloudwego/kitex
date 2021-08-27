# Transport Protocol TTheader

Referring to the [Thrift THeader protocol](https://github.com/apache/thrift/blob/master/doc/specs/HeaderFormat.md), we designed the TTheader protocol.

## Header Format

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

- `LENGTH`: 32bits, including the byte size of the remaining part of the data packet, but does not include itself
- `HEADER MAGIC`: 16bits, value: `0x1000`, it's used to identify `TTHeaderTransport`
- `FLAGS`: 16bits, it's a reserved field, not used yet, default value is `0x0000`
- `SEQUENCE NUMBER`: 32bits, it's the seqId of the message packet, can be used for multiplexing, and ensure it is incremented within a single connection
- `HEADER SIZE`: 16bits, it's equal to the number of bytes divided by 4 of the head length. The header length calculation starts from the 14th byte and continues until before `PAYLOAD` (Notice: The maximum length of the header is 64K)
- `PROTOCOL ID`: uint8 encoding, the values are:
  - `ProtocolIDBinary = 0`
  - `ProtocolIDCompact = 2`
- `NUM TRANSFORMS`: uint8 encoding, the number of `TRANSFORM`
- `TRANSFORM ID`: uint8 encoding, refer the instructions below
- `INFO ID`: uint8 encoding, refer the instructions below
- `PAYLOAD`: message content

## PADDING

Header will be padded out to next 4-byte boundary with `0x00`.

## Transform IDs

`Transform IDs` represents the compression way, which is a reserved field and is not supported currently. The values are:

- `ZLIB_TRANSFORM = 0x01`: the corresponding data is empty, indicating that the data is compressed with zlib
- `SNAPPY_TRANSFORM = 0x03`: the corresponding data is empty, indicating that the data is compressed with snappy

## Info IDs

`Info IDs` is used to transmit some key/value pair information, the values are:

- `INFO_KEYVALUE = 0x01`: the corresponding data is a key/value pair, key and value are each composed of the length of uint16 plus no-training-null string, which is generally used to transmit some common meta information, such as tracingId
- `INFO_INTKEYVALUE = 0x10`: the corresponding data is a key/value pair, the key is uint16, the value is composed of the length of uint16 plus the no-trading-null string, which is generally used to transmit some internally customized meta information, some keys are required as request:
  - `TRANSPORT_TYPE = 1` (value: framed/unframed)
  - `LOG_ID = 2`
  - `FROM_SERVICE = 3`
  - `FROM_CLUSTER = 4` (default)
  - `FROM_IDC = 5`
  - `TO_SERVICE = 6`
  - `TO_METHOD = 9`
- `ACL_TOKEN_KEYVALUE = 0x11`: the corresponding data is a key/value pair, key and value are each composed of the length of uint16 plus no-trading-null string, used to transmit ACL Token