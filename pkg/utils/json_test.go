/*
 * Copyright 2021 CloudWeGo Authors
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

package utils

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	jsoniter "github.com/json-iterator/go"
	"github.com/stretchr/testify/require"

	"github.com/cloudwego/kitex/internal/test"
)

func changeJSONLib(sonic bool) {
	if sonic {
		Map2JSONStr = _Map2JSONStr_sonic
		JSONStr2Map = _JSONStr2Map_sonic
	} else {
		Map2JSONStr = _Map2JSONStr_old
		JSONStr2Map = _JSONStr2Map_old
	}
}

func FuzzJSONStr2Map(f *testing.F) {
	mapInfo := prepareMap()
	jsonRet, _ := jsoni.MarshalToString(mapInfo)
	f.Add(`null`)
	f.Add(`{}`)
	f.Add(`{"":""}`)
	f.Add(jsonRet)
	f.Add(`{"\u4f60\u597d": "\u5468\u6770\u4f26"}`)
	f.Fuzz(func(t *testing.T, data string) {
		if !json.Valid([]byte(data)) {
			return
		}
		changeJSONLib(true)
		map1, err1 := _JSONStr2Map_sonic(data)
		changeJSONLib(false)
		map2, err2 := _JSONStr2Map_old(data)
		require.Equal(t, err2 == nil, err1 == nil, "json: %v", data)
		if err2 == nil {
			require.Equal(t, map2, map1, "json:%v", data)
		}
	})
}

func FuzzMap2JSON(f *testing.F) {
	mapInfo := prepareMap()
	jsonRet, _ := jsoni.Marshal(mapInfo)
	f.Add([]byte(`null`))
	f.Add([]byte(`{}`))
	f.Add([]byte(`{"":""}`))
	f.Add([]byte(jsonRet))
	f.Add([]byte(`{"\u4f60\u597d": "\u5468\u6770\u4f26"}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		m := map[string]string{}
		if err := json.Unmarshal(data, &m); err != nil {
			return
		}
		changeJSONLib(true)
		map1, err1 := _Map2JSONStr_sonic(m)
		changeJSONLib(false)
		map2, err2 := _Map2JSONStr_old(m)
		require.Equal(t, err2 == nil, err1 == nil, "json: %v", data)
		if err2 == nil {
			m2 := map[string]string{}
			if err := json.Unmarshal([]byte(map1), &m2); err != nil {
				return
			}
			m3 := map[string]string{}
			if err := json.Unmarshal([]byte(map2), &m3); err != nil {
				return
			}
			require.Equal(t, m2, m3, "json:%v", data)
		}
	})
}

var jsoni = jsoniter.ConfigCompatibleWithStandardLibrary

var samples = []struct {
	name string
	m    map[string]string
}{
	{"10x", prepareMap2(10, 10, 10, 4)},
	{"100x", prepareMap2(10, 100, 100, 4)},
	{"1000x", prepareMap2(10, 1000, 1000, 4)},
}

func BenchmarkMap2JSONStr(b *testing.B) {
	for _, s := range samples {
		b.Run(s.name, func(b *testing.B) {
			_, _ = _Map2JSONStr_sonic(s.m)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, _ = _Map2JSONStr_sonic(s.m)
			}
		})
	}
}

func BenchmarkJSONStr2Map(b *testing.B) {
	for _, s := range samples {
		b.Run(s.name, func(b *testing.B) {
			j, _ := jsoni.MarshalToString(s.m)
			_, _ = _JSONStr2Map_sonic(j)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, _ = _JSONStr2Map_sonic(j)
			}
		})
	}
}

func BenchmarkJSONMarshal(b *testing.B) {
	mapInfo := prepareMap()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		jsonMarshal(mapInfo)
	}
}

func BenchmarkJSONUnmarshal(b *testing.B) {
	mapInfo := prepareMap()
	jsonRet, _ := jsoni.MarshalToString(mapInfo)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var map1 map[string]string
		json.Unmarshal([]byte(jsonRet), &map1)
	}
}

func BenchmarkJSONIterMarshal(b *testing.B) {
	mapInfo := prepareMap()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		jsoni.MarshalToString(mapInfo)
	}
}

func BenchmarkJSONIterUnmarshal(b *testing.B) {
	mapInfo := prepareMap()
	jsonRet, _ := Map2JSONStr(mapInfo)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var map1 map[string]string
		jsoni.UnmarshalFromString(jsonRet, &map1)
	}
}

// TestMap2JSONStr test convert map to json string
func TestMap2JSONStr(t *testing.T) {
	mapInfo := prepareMap()

	jsonRet1, err := Map2JSONStr(mapInfo)
	test.Assert(t, err == nil)
	jsonRet2, _ := json.Marshal(mapInfo)

	// 用Unmarshal解序列化两种方式的jsonStr
	var map1 map[string]string
	json.Unmarshal([]byte(jsonRet1), &map1)
	var map2 map[string]string
	json.Unmarshal(jsonRet2, &map2)

	test.Assert(t, len(map1) == len(map2))
	for k := range map1 {
		test.Assert(t, map1[k] == map2[k])
	}

	mapInfo = nil
	jsonRetNil, err := Map2JSONStr(mapInfo)
	test.Assert(t, err == nil)
	test.Assert(t, jsonRetNil == "{}", jsonRetNil)

	mapRet := map[string]string{
		"\u4f60\u597d": "\u5468\u6770\u4f26",
	}
	act, err := Map2JSONStr(mapRet)
	test.Assert(t, err == nil)
	test.Assert(t, act == `{"你好":"周杰伦"}`)

	key := []byte("\u4f60\u597d")
	val := []byte("\u5468\u6770\u4f26")
	illegalUnicodeMapRet := map[string]string{
		SliceByteToString(key): SliceByteToString(val),
	}
	key[0] = 255
	key[1] = 255
	val[0] = 255
	val[1] = 255
	act, err = Map2JSONStr(illegalUnicodeMapRet)
	test.Assert(t, err == nil, err.Error())
	test.Assert(t, act == `{"\uffd好":"\uffd杰伦"}`)
}

// TestJSONStr2Map test convert json string to map
func TestJSONStr2Map(t *testing.T) {
	mapInfo := prepareMap()
	jsonRet, _ := json.Marshal(mapInfo)
	println(string(jsonRet))
	mapRet, err := JSONStr2Map(string(jsonRet))
	test.Assert(t, err == nil)

	test.Assert(t, len(mapInfo) == len(mapRet))
	for k := range mapInfo {
		test.Assert(t, mapInfo[k] == mapRet[k])
	}

	str := "null"
	ret, err := JSONStr2Map(str)
	test.Assert(t, ret == nil)
	test.Assert(t, err == nil)
	str = "nUll"
	ret, err = JSONStr2Map(str)
	test.Assert(t, ret == nil)
	test.Assert(t, err != nil)
	str = "nuLL"
	ret, err = JSONStr2Map(str)
	test.Assert(t, ret == nil)
	test.Assert(t, err != nil)
	str = "nulL"
	ret, err = JSONStr2Map(str)
	test.Assert(t, ret == nil)
	test.Assert(t, err != nil)

	str = "{}"
	ret, err = JSONStr2Map(str)
	test.Assert(t, ret == nil)
	test.Assert(t, err == nil)

	str = "{"
	ret, err = JSONStr2Map(str)
	test.Assert(t, ret == nil)
	test.Assert(t, err != nil)

	unicodeStr := `{"\u4f60\u597d": "\u5468\u6770\u4f26"}`
	mapRet, err = JSONStr2Map(unicodeStr)
	test.Assert(t, err == nil)
	test.Assert(t, mapRet["你好"] == "周杰伦")

	unicodeMixedStr := `{"aaa\u4f60\u597daaa": ":\\\u5468\u6770\u4f26"  ,  "加油 \u52a0\u6cb9" : "Come on \u52a0\u6cb9"}`
	mapRet, err = JSONStr2Map(unicodeMixedStr)
	test.Assert(t, err == nil)
	test.Assert(t, mapRet["aaa你好aaa"] == ":\\周杰伦")
	test.Assert(t, mapRet["加油 加油"] == "Come on 加油")

	unicodeMixedStr = `{"aaa\u4F60\u597Daaa": "\u5468\u6770\u4f26"}`
	mapRet, err = JSONStr2Map(unicodeMixedStr)
	test.Assert(t, err == nil)
	test.Assert(t, mapRet["aaa你好aaa"] == "周杰伦")

	illegalUnicodeStr := `{"a":"b", "aaa\u4z60\u597daaa": "加油"}`
	illegalUnicodeMapRet, err := JSONStr2Map(illegalUnicodeStr)
	test.Assert(t, err != nil)
	test.Assert(t, len(illegalUnicodeMapRet) == 0)

	surrogateUnicodeStr := `{"a":"b", "\u4F60\u597D": "\uDFFF\uD800"}`
	surrogateUnicodeMapRet, err := JSONStr2Map(surrogateUnicodeStr)
	test.Assert(t, err == nil)
	test.Assert(t, surrogateUnicodeMapRet != nil)

	illegalEscapeCharStr := `{"a":"b", "\x4F60\x597D": "\uDFqwdFF\uD800"}`
	illegalEscapeCharMapRet, err := JSONStr2Map(illegalEscapeCharStr)
	test.Assert(t, err != nil)
	test.Assert(t, len(illegalEscapeCharMapRet) == 0)
}

// TestJSONUtil compare return between encoding/json, json-iterator and json.go
func TestJSONUtil(t *testing.T) {
	mapInfo := prepareMap()
	jsonRet1, _ := json.Marshal(mapInfo)
	jsonRet2, _ := jsoni.MarshalToString(mapInfo)
	jsonRet, _ := Map2JSONStr(mapInfo)

	var map1 map[string]string
	json.Unmarshal(jsonRet1, &map1)
	var map2 map[string]string
	json.Unmarshal([]byte(jsonRet2), &map2)
	var map3 map[string]string
	json.Unmarshal([]byte(jsonRet), &map3)

	test.Assert(t, len(map1) == len(map3))
	test.Assert(t, len(map2) == len(map3))
	for k := range map1 {
		test.Assert(t, map2[k] == map3[k])
	}

	mapRetIter := make(map[string]string)
	jsoni.UnmarshalFromString(string(jsonRet1), &mapRetIter)
	mapRet, err := JSONStr2Map(string(jsonRet1))
	test.Assert(t, err == nil)

	test.Assert(t, len(mapRet) == len(mapRetIter))
	test.Assert(t, len(mapRet) == len(map1))

	for k := range map1 {
		test.Assert(t, mapRet[k] == mapRetIter[k])
	}
}

func prepareMap() map[string]string {
	mapInfo := make(map[string]string)
	mapInfo["a"] = "a"
	mapInfo["bb"] = "bb"
	mapInfo["ccc"] = "ccc"
	mapInfo["env"] = "test"
	mapInfo["destService"] = "kite.server"
	mapInfo["remoteIP"] = "10.1.2.100"
	mapInfo["hello"] = "hello"
	mapInfo["fongojannfsoaigsang ojosaf"] = "fsdaufsdjagnji91nngnajjmfsaf"
	mapInfo["公司"] = "字节"

	mapInfo["a\""] = "b"
	mapInfo["c\""] = "\"d"
	mapInfo["e\"ee"] = "ff\"f"
	mapInfo["g:g,g"] = "h,h:hh"
	mapInfo["i:\a\"i"] = "g\"g:g"
	mapInfo["k:\\k\":\"\\\"k"] = "l\\\"l:l"
	mapInfo["m, m\":"] = "n	n\":\"n"
	mapInfo["m, &m\":"] = "n<!	>n\":\"n"
	mapInfo[`{"aaa\u4f60\u597daaa": ":\\\u5468\u6770\u4f26"  ,  "加油 \u52a0\u6cb9" : "Come on \u52a0\u6cb9"}`] = `{"aaa\u4f60\u597daaa": ":\\\u5468\u6770\u4f26"  ,  "加油 \u52a0\u6cb9" : "Come on \u52a0\u6cb9"}`
	return mapInfo
}

func prepareMap2(keyLen, valLen, count, escaped int) map[string]string {
	mapInfo := make(map[string]string, count)
	for i := 0; i < count; i++ {
		key := strings.Repeat("a", keyLen) + strconv.Itoa(i)
		if i%escaped == 0 {
			// get escaped
			key += ","
		}
		val := strings.Repeat("b", keyLen)
		if i%escaped == 0 {
			// get escaped
			val += ","
		}
		mapInfo[string(key)] = string(val)
	}
	return mapInfo
}

func jsonMarshal(userExtraMap map[string]string) (string, error) {
	ret, err := json.Marshal(userExtraMap)
	return string(ret), err
}
