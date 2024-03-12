//go:build go1.18
// +build go1.18

/*
 * Copyright 2024 CloudWeGo Authors
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
	"testing"

	"github.com/stretchr/testify/require"
)

func FuzzJSONStr2Map(f *testing.F) {
	mapInfo := prepareMap()
	jsonRet, _ := jsoni.MarshalToString(mapInfo)
	f.Add(`{}`)
	f.Add(`{"":""}`)
	f.Add(jsonRet)
	f.Add(`{"\u4f60\u597d": "\u5468\u6770\u4f26"}`)
	f.Add(`{"aaa\u4f60\u597daaa": ":\\\u5468\u6770\u4f26"  ,  "加油 \u52a0\u6cb9" : "Come on \u52a0\u6cb9"}`)
	f.Add(`{"\u4F60\u597D": "\uDFFF\uD800"}`)
	f.Add(`{"a":"b", "\x4F60\x597D": "\uDFqwdFF\uD800"}`)
	f.Fuzz(func(t *testing.T, data string) {
		if !json.Valid([]byte(data)) {
			return
		}
		map1, err1 := JSONStr2Map(data)
		map2, err2 := _JSONStr2Map(data)
		require.Equal(t, err2 == nil, err1 == nil, "json:%v", data)
		if err2 == nil {
			require.Equal(t, map2, map1, "json:%v", data)
		}
	})
}

func FuzzMap2JSON(f *testing.F) {
	mapInfo := prepareMap()
	jsonRet, _ := jsoni.MarshalToString(mapInfo)
	f.Add(`{}`)
	f.Add(`{"":""}`)
	f.Add(jsonRet)
	f.Add(`{"\u4f60\u597d": "\u5468\u6770\u4f26"}`)
	f.Add(`{"aaa\u4f60\u597daaa": ":\\\u5468\u6770\u4f26"  ,  "加油 \u52a0\u6cb9" : "Come on \u52a0\u6cb9"}`)
	f.Add(`{"\u4F60\u597D": "\uDFFF\uD800"}`)
	f.Add(`{"a":"b", "\x4F60\x597D": "\uDFqwdFF\uD800"}`)
	f.Fuzz(func(t *testing.T, data string) {
		var m map[string]string
		if err := json.Unmarshal([]byte(data), &m); err != nil {
			return
		}
		map1, err1 := Map2JSONStr(m)
		map2, err2 := _Map2JSONStr(m)
		require.Equal(t, err2 == nil, err1 == nil, "json:%v", data)
		require.Equal(t, len(map2), len(map1), "json:%v", data)
		var m1, m2 map[string]string
		require.NoError(t, json.Unmarshal([]byte(map1), &m1))
		require.NoError(t, json.Unmarshal([]byte(map2), &m2))
		require.Equal(t, m2, m1)
	})
}
