namespace go test

enum AEnum {
  A = 1,
  B = 2,
}

struct Inner {
  1: optional i32 Num = 5,
  2: optional string desc,
  3: optional map<i64, list<i64>> MapOfList,
  4: optional map<AEnum, i64> MapOfEnumKey,
  5: optional byte Byte1,
  6: optional double Double1,
}

enum HTTPStatus {
  OK = 200,
  NOT_FOUND = 404
}

struct local {
  1: i32 l,
}

struct local2 {
  1: optional local l1,
}

struct TestStruct {
  1: required i32 Left,
  2: optional i32 Right = 3,
  3: binary Dummy,
  4: Inner InnerReq,
  5: HTTPStatus status,
  6: string Str,
  7: list<HTTPStatus> enum_list,
  8: optional map<i32, string> Strmap,
  9: i64 Int64,
  10: optional list<i32> IntList,
  11: list<local> localList,
  12: map<string, local> StrLocalMap,
  13: list<list<i32>> nestList,
  14: required local required_ins,
  # 15: map<local, string> com_map_key,
  16: map<string, list<string>> nestMap,
  17: list<map<string, HTTPStatus>> nestMap2,
  18: map<i32, HTTPStatus> enum_map,
  19: list<string> Strlist,
  20: optional local optional_ins,
  21: Inner AnotherInner,
  22: optional list<string> opt_nil_list,
  23: list<string> nil_list,
  24: optional list<Inner> opt_nil_ins_list,
  25: list<Inner> nil_ins_list,
  26: optional HTTPStatus opt_status,
  27: map<HTTPStatus, local> enum_key_map,
  28: map<HTTPStatus, list<map<string, local>>> complex,
}

struct EmptyStruct {
}

