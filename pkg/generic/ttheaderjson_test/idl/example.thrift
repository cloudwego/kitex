namespace go thrift

struct Request {
    1: i32 Type,
    2: string Message,
}

struct Response {
    1: i32 Type,
    2: string Message,
}

service TestService {
    Response Unary (1: Request req) (streaming.mode="unary")
    Response ClientStream (1: Request req) (streaming.mode="client"),
    Response ServerStream (1: Request req) (streaming.mode="server"),
    Response BidiStream (1: Request req) (streaming.mode="bidirectional"),
    Response UnaryWithErr (1: Request req) (streaming.mode="unary"),
    Response ClientStreamWithErr (1: Request req) (streaming.mode="client"),
    Response ServerStreamWithErr (1: Request req) (streaming.mode="server"),
    Response BidiStreamWithErr (1: Request req) (streaming.mode="bidirectional"),

    Response PingPong (1: Request req), // KitexThrift, non-streaming
}