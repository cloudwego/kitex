namespace go thrift

struct MockReq {
	1: string Msg,
	2: map<string, string> strMap,
	3: list<string> strList,
}

service Mock {
    string Test(1:MockReq req)
}


