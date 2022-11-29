namespace go kitex.test.server

struct ReqItem{
    1: optional i64 id(api.js_conv = '')
    2: optional string text
}

struct BinaryWrapper {
    1: binary msg (api.body = "msg")
    2: bool got_base64 (api.body = "got_base64")
    3: i64 num (api.body = "num", api.js_conv="")
    4: ReqItem some(api.body = 'some')
}

service ExampleService {
    BinaryWrapper BinaryEcho(1: BinaryWrapper req) (api.get = '/BinaryEcho', api.baseurl = 'example.com')
}