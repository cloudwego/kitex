/*
 * Copyright 2023 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

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

struct Local {
  1: i32 l,
}

struct FullStruct {
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
  11: list<Local> localList,
  12: map<string, Local> StrLocalMap,
  13: list<list<i32>> nestList,
  14: required Local required_ins,
  16: map<string, list<string>> nestMap,
  17: list<map<string, HTTPStatus>> nestMap2,
  18: map<i32, HTTPStatus> enum_map,
  19: list<string> Strlist,
  20: optional Local optional_ins,
  21: Inner AnotherInner,
  22: optional list<string> opt_nil_list,
  23: list<string> nil_list,
  24: optional list<Inner> opt_nil_ins_list,
  25: list<Inner> nil_ins_list,
  26: optional HTTPStatus opt_status,
  27: map<HTTPStatus, Local> enum_key_map,
  28: map<HTTPStatus, list<map<string, Local>>> complex,
}

struct MixedStruct {
  1: required i32 Left,
  3: binary Dummy,
  6: string Str,
  7: list<HTTPStatus> enum_list,
  9: i64 Int64,
  10: optional list<i32> IntList,
  11: list<Local> localList,
  12: map<string, Local> StrLocalMap,
  13: list<list<i32>> nestList,
  14: required Local required_ins,
  20: optional Local optional_ins,
  21: Inner AnotherInner,
  27: map<HTTPStatus, Local> enum_key_map,
}

struct EmptyStruct {
}

