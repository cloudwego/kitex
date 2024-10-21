struct Request {
    1: required string Msg
}

struct Response {
    1: optional i8 I8 = 8,
    2: optional Test Test = {"b": "bbb"},
    3: optional Test TestWithNoVal,
    4: optional map<string, i16> Map = {"one":1, "two":2},
    5: optional list<Test> TestSet = [{"a":"a"},{"b":"b"},{"c":1.23}],
    6: required string Msg
}

struct Test {
    1: optional string a = "aaa"
    2: string b
    3: double c
}

service ServiceA {
    Response Echo(1: Request req)
}