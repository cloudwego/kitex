namespace go kitex.test.server

struct BinaryWrapper {
    1: binary msg (api.body = "msg")
    2: bool got_base64 (api.body = "got_base64")
}

service ExampleService {
    BinaryWrapper BinaryEcho(1: BinaryWrapper req) (api.get = '/BinaryEcho', api.baseurl = 'example.com')
}