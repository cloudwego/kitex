/*
 * Copyright 2025 CloudWeGo Authors
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

package idl_info

import (
	"os"
	"testing"

	"github.com/cloudwego/kitex/internal/test"
)

func TestRegisterThriftIDL(t *testing.T) {
	defer resetGlobalManager()

	// register group 1
	test.Assert(t, RegisterThriftIDL("idl_service_1", "mod1", "12345678", true, "aaa", "./1/a.thrift") == nil)
	test.Assert(t, RegisterThriftIDL("idl_service_1", "mod1", "12345678", false, "bbb", "./1/b.thrift") == nil)
	test.Assert(t, RegisterThriftIDL("idl_service_1", "mod1", "12345678", false, "ccc", "./1/c.thrift") == nil)
	// register group 2
	test.Assert(t, RegisterThriftIDL("idl_service_2", "mod2", "23456789", true, "ddd", "./2/d.thrift") == nil)
	test.Assert(t, RegisterThriftIDL("idl_service_2", "mod2", "23456789", false, "eee", "./2/e.thrift") == nil)
	test.Assert(t, RegisterThriftIDL("idl_service_2", "mod2", "23456789", false, "fff", "./2/f.thrift") == nil)

	metas := getAllIDLInfoMetas()
	test.Assert(t, len(metas) == 2)

	info1, ok := getInfoFromManager("idl_service_1")
	test.Assert(t, ok)
	test.Assert(t, info1.IDLServiceName == "idl_service_1")
	test.Assert(t, info1.IDLCommitID == "12345678")
	test.Assert(t, info1.MainIDLPath == "./1/a.thrift")
	test.Assert(t, info1.GoModule == "mod1")
	test.Assert(t, len(info1.IDLFileContentMap) == 3)
	test.Assert(t, info1.IDLFileContentMap["./1/a.thrift"] == "aaa")
	test.Assert(t, info1.IDLFileContentMap["./1/b.thrift"] == "bbb")
	test.Assert(t, info1.IDLFileContentMap["./1/c.thrift"] == "ccc")

	info2, ok := getInfoFromManager("idl_service_2")
	test.Assert(t, ok)
	test.Assert(t, info2.IDLServiceName == "idl_service_2")
	test.Assert(t, info2.IDLCommitID == "23456789")
	test.Assert(t, info2.MainIDLPath == "./2/d.thrift")
	test.Assert(t, info2.GoModule == "mod2")
	test.Assert(t, len(info2.IDLFileContentMap) == 3)
	test.Assert(t, info2.IDLFileContentMap["./2/d.thrift"] == "ddd")
	test.Assert(t, info2.IDLFileContentMap["./2/e.thrift"] == "eee")
	test.Assert(t, info2.IDLFileContentMap["./2/f.thrift"] == "fff")

	_, ok = getInfoFromManager("idl_service_3")
	test.Assert(t, !ok)

	test.Assert(t, RegisterThriftIDL("idl_service_3", "mod1", "12345678", true, "aaa", "./1/a.thrift") == nil)
	// test conflict
	test.Assert(t, RegisterThriftIDL("idl_service_3", "mod2", "23456789", true, "ddd", "./2/d.thrift") != nil)
	// set environment to skip conflict
	test.Assert(t, os.Setenv("KITEX_IDL_INFO_DISABLE_CONFLICT_CHECK", "1") == nil)
	// register conflict again with 'skip environment'
	test.Assert(t, RegisterThriftIDL("idl_service_3", "mod2", "23456789", true, "ddd", "./2/d.thrift") == nil)
	// there should be only one idl registered in idl_service_3
	metas = getAllIDLInfoMetas()
	test.Assert(t, len(metas) == 3)
	// check details
	info3, ok := getInfoFromManager("idl_service_3")
	test.Assert(t, ok)
	test.Assert(t, info3.IDLServiceName == "idl_service_3")
	test.Assert(t, info3.IDLCommitID == "12345678")
	test.Assert(t, info3.MainIDLPath == "./1/a.thrift")
	test.Assert(t, info3.GoModule == "mod1")
	test.Assert(t, len(info3.IDLFileContentMap) == 1)
	test.Assert(t, info3.IDLFileContentMap["./1/a.thrift"] == "aaa")
}

func TestIDLInfoAPIs(t *testing.T) {
	defer resetGlobalManager()
	// register group 1
	test.Assert(t, RegisterThriftIDL("idl_service_1", "mod1", "12345678", true, "aaa", "./1/a.thrift") == nil)
	test.Assert(t, RegisterThriftIDL("idl_service_1", "mod1", "12345678", false, "bbb", "./1/b.thrift") == nil)
	test.Assert(t, RegisterThriftIDL("idl_service_1", "mod1", "12345678", false, "ccc", "./1/c.thrift") == nil)
	// register group 2
	test.Assert(t, RegisterThriftIDL("idl_service_2", "mod2", "23456789", true, "ddd", "./2/d.thrift") == nil)
	test.Assert(t, RegisterThriftIDL("idl_service_2", "mod2", "23456789", false, "eee", "./2/e.thrift") == nil)
	test.Assert(t, RegisterThriftIDL("idl_service_2", "mod2", "23456789", false, "fff", "./2/f.thrift") == nil)

	info1, ok := GetIDLInfoDetails("idl_service_1")
	test.Assert(t, ok)
	test.Assert(t, info1.IDLServiceName == "idl_service_1")
	test.Assert(t, info1.IDLCommitID == "12345678")
	test.Assert(t, info1.MainIDLPath == "./1/a.thrift")
	test.Assert(t, info1.GoModule == "mod1")
	test.Assert(t, len(info1.IDLFileContentMap) == 3)
	test.Assert(t, info1.IDLFileContentMap["./1/a.thrift"] == "aaa")
	test.Assert(t, info1.IDLFileContentMap["./1/b.thrift"] == "bbb")
	test.Assert(t, info1.IDLFileContentMap["./1/c.thrift"] == "ccc")

	info2, ok := GetIDLInfoDetails("idl_service_2")
	test.Assert(t, ok)
	test.Assert(t, info2.IDLServiceName == "idl_service_2")
	test.Assert(t, info2.IDLCommitID == "23456789")
	test.Assert(t, info2.MainIDLPath == "./2/d.thrift")
	test.Assert(t, info2.GoModule == "mod2")
	test.Assert(t, len(info2.IDLFileContentMap) == 3)
	test.Assert(t, info2.IDLFileContentMap["./2/d.thrift"] == "ddd")
	test.Assert(t, info2.IDLFileContentMap["./2/e.thrift"] == "eee")
	test.Assert(t, info2.IDLFileContentMap["./2/f.thrift"] == "fff")

	_, ok = GetIDLInfoDetails("idl_service_3")
	test.Assert(t, !ok)

	// test modify the result, the result should be unchangeable
	metas := GetIDLInfoList()
	test.Assert(t, len(metas) == 2)
	for i := range metas {
		info, ok := GetIDLInfoDetails(metas[i].IDLServiceName)
		test.Assert(t, ok)
		test.Assert(t, len(info.IDLFileContentMap) == 3)
		metas[i].IDLType = "xxxx"
		info.IDLFileContentMap["xxx"] = "xxx"
	}
	metas2 := GetIDLInfoList()
	test.Assert(t, len(metas2) == 2)
	for i := range metas2 {
		test.Assert(t, metas2[i].IDLType == "thrift")
		info, ok := GetIDLInfoDetails(metas2[i].IDLServiceName)
		test.Assert(t, ok)
		test.Assert(t, len(info.IDLFileContentMap) == 3)
	}
}
