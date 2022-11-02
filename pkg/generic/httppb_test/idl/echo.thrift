namespace go kitex.test.server

enum TestEnum {
     Zero = 0
     First = 1
}

struct Elem {
    1: bool ok
}

struct Message {
    1: i8 tiny
    2: i64 large
    3: TestEnum tenum
    4: string str
    5: map<string, Elem> elems
    6: list<Elem> els
    // 7: map<i32, Elem> els = {12: {"ok": true}}
}

service ExampleService {
    Message Echo(1: Message req) (api.get = '/Echo', api.baseurl = 'example.com')
}
