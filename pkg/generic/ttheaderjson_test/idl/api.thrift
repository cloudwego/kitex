namespace go thrift

struct Request {
    1: required string message,
}

struct Response {
    1: required string message,
}

service TestService {
    Response Echo (1: Request req) (streaming.mode="bidirectional"),
    Response EchoClient (1: Request req) (streaming.mode="client"),
    Response EchoServer (1: Request req) (streaming.mode="server"),
    Response EchoUnary (1: Request req) (streaming.mode="unary"), // not recommended
    Response EchoBizException (1: Request req) (streaming.mode="client"),

    Response EchoPingPong (1: Request req), // KitexThrift, non-streaming
}
