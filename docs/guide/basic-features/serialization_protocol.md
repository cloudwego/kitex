# Serialization Protocol

Kitex support had support two serialization protocol: Thrift and Protobuf.

## Thrift

Kitex only support Thrift [Binary](https://github.com/apache/thrift/blob/master/doc/specs/thrift-binary-protocol.md) protocol codec, [Compact](https://github.com/apache/thrift/blob/master/doc/specs/thrift-compact-protocol.md) currently is not supported.

If you want using thrift protocol encoding, should generate codes by kitex cmd:

Client side：

```
kitex -type thrift ${service_name} ${idl_name}.thrift
```

Server side:

 ```
kitex -type thrift -service ${service_name} ${idl_name}.thrift
 ```

We have optimized Thrift's Binary protocol codec. For details of the optimization, please refer to the "Reference - High Performance Thrift Codec" chapter. If you want to close these optimizations, you can add the `-no-fast-api` argument when generating code.

## Protobuf

### Protocol Type

There are two types suporting of protobuf:

1. **Custom message protocol**: it's been considered as kitex protobuf, the way of generated code is consistent with Thrift.
2. **gRPC protocol**: it can communication with grpc directly, and support streaming.

If the streaming method is defined in the IDL, the serialization protocol would adopt gRPC protocol, otherwise Kitex protobuf would be adopted. If you want using gRPC protocol, but without stream definition in your proto file, you need specify the transport protocol when initializing client (No changes need to be made on the server because protocol detection is supported)：

```go
// Using WithTransportProtocol specify the transport protocol
cli, err := service.NewClient(destService, client.WithTransportProtocol(transport.GRPC))
```

### Generated Code

Only support proto3, the grammar reference: https://developers.google.com/protocol-buffers/docs/gotutorial.

Notice:

1. What is different from other languages, generating go codes must define `go_package` in the proto file
2. Instead of the full path, just using `go_package` specify the package name, such as: go_package = "pbdemo"
3. Download the `protoc` binary and put it in the $PATH directory

Client side：

```
kitex -type protobuf -I idl/ idl/${proto_name}.proto
```

Server side:

```
kitex -type protobuf -service ${service_name} -I idl/ idl/${proto_name}.proto
```