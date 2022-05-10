namespace go kitex.test.server

struct BinaryWrapper {
    1: binary msg (api.body = "msg")
    2: bool got_base64 (api.body = "got_base64")
}

struct ListWrapper {
    1: list<string> userIds (api.body = "user_ids")
}

service ExampleService {
    BinaryWrapper BinaryEcho(1: BinaryWrapper req) (api.get = '/BinaryEcho', api.baseurl = 'example.com')

    ListWrapper ListEcho(1: ListWrapper req)(api.get = '/ListEcho', api.baseurl = 'example.com')
}
