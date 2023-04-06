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

package thriftreflection

import (
	"context"
	"errors"
	"strings"

	"github.com/cloudwego/thriftgo/parser"

	"github.com/cloudwego/kitex/pkg/serviceinfo"
)

// ReflectionMethod name
var ReflectionMethod = "$KitexReflectionMethod"

var reflectionMethodInfo = serviceinfo.NewMethodInfo(func(ctx context.Context, handler, arg, result interface{}) error {
	realArg := arg.(*ReflectionServiceKitexReflectionQueryIDLArgs)
	realResult := result.(*ReflectionServiceKitexReflectionQueryIDLResult)
	req, err := DecodeQueryIDLRequest(realArg.Req)
	if err != nil {
		return err
	}
	resp, err := QueryThriftIDL(req)
	if err != nil {
		return err
	}
	success, err := resp.JsonEncode()
	if err != nil {
		return err
	}
	realResult.Success = &success
	return nil
}, func() interface{} {
	return NewReflectionServiceKitexReflectionQueryIDLArgs()
}, func() interface{} {
	return NewReflectionServiceKitexReflectionQueryIDLResult()
}, false)

func RegisterReflectionMethod(svcInfo *serviceinfo.ServiceInfo) {
	if svcInfo == nil {
		return
	}
	methods := make(map[string]serviceinfo.MethodInfo, len(svcInfo.Methods)+1)
	for k, v := range svcInfo.Methods {
		methods[k] = v
	}
	methods[ReflectionMethod] = reflectionMethodInfo
	svcInfo.Methods = methods
}

func QueryThriftIDL(req *QueryIDLRequest) (resp *QueryIDLResponse, err error) {
	if req.QueryType == "QueryService" {
		resp, err = QueryServiceIDL(req)
	} else if req.QueryType == "BatchQueryServices" {
		resp, err = BatchQueryServiceIDL(req)
	} else if req.QueryType == "BatchQueryMethods" {
		resp, err = BatchQueryMethodsIDL(req)
	} else {
		return nil, errors.New("No QueryType for:" + req.QueryType)
	}
	return
}

func QueryServiceIDL(req *QueryIDLRequest) (resp *QueryIDLResponse, err error) {
	svcName := req.QueryInput
	methods := map[string]*MethodDescriptor{}
	usedStructs := map[string]*StructDescriptor{}
	sd := &ServiceDescriptor{
		Methods:     methods,
		UsedStructs: usedStructs,
	}

	path, svc, svcExist := findSvcFromAll(svcName)
	if !svcExist {
		return nil, errors.New("No service found by given name:" + svcName)
	}

	for _, m := range svc.Functions {
		md := NewMethodDescriptor(path, m)
		methods[md.String()] = md
	}

	if len(svc.Extends) > 0 {
		hasPrefix, prefix, rawName := parseName(svc.Extends)
		findPath := path
		if hasPrefix {
			findPath = prefixToPath(path, prefix)
		}
		extSvc, ok := findSvc(findPath, rawName)
		if ok {
			for _, m := range extSvc.Functions {
				md := NewMethodDescriptor(findPath, m)
				methods[md.String()] = md
			}
		}
	}

	for idx, m := range methods {
		is := m.IncludedStructs
		for k, v := range is {
			// only record names
			methods[idx].IncludedStructs[k] = nil
			usedStructs[k] = NewStructDescriptor(k, v)
		}
	}

	resp = &QueryIDLResponse{ServiceInfo: sd}

	return
}

func BatchQueryServiceIDL(req *QueryIDLRequest) (resp *QueryIDLResponse, err error) {
	svcNames := strings.Split(req.QueryInput, ",")
	methods := map[string]*MethodDescriptor{}
	usedStructs := map[string]*StructDescriptor{}
	sd := &ServiceDescriptor{
		Methods:     methods,
		UsedStructs: usedStructs,
	}

	for _, svcName := range svcNames {
		path, svc, svcExist := findSvcFromAll(svcName)
		if !svcExist {
			return nil, errors.New("No service found by given name:" + svcName)
		}

		for _, m := range svc.Functions {
			md := NewMethodDescriptor(path, m)
			methods[md.String()] = md
		}

		if len(svc.Extends) > 0 {
			hasPrefix, prefix, rawName := parseName(svc.Extends)
			findPath := path
			if hasPrefix {
				findPath = prefixToPath(path, prefix)
			}
			extSvc, ok := findSvc(findPath, rawName)
			if ok {
				for _, m := range extSvc.Functions {
					md := NewMethodDescriptor(findPath, m)
					methods[md.String()] = md
				}
			}
		}

		for idx, m := range methods {
			is := m.IncludedStructs
			for k, v := range is {
				// only record names
				methods[idx].IncludedStructs[k] = nil
				usedStructs[k] = NewStructDescriptor(k, v)
			}
		}
	}

	resp = &QueryIDLResponse{ServiceInfo: sd}

	return
}

func BatchQueryMethodsIDL(req *QueryIDLRequest) (resp *QueryIDLResponse, err error) {
	methodNames := strings.Split(req.QueryInput, ",")
	methods := map[string]*MethodDescriptor{}
	usedStructs := map[string]*StructDescriptor{}
	sd := &ServiceDescriptor{
		Methods:     methods,
		UsedStructs: usedStructs,
	}

	for _, mName := range methodNames {
		path, m, methodExist := findMethodFromAll(mName)
		if !methodExist {
			continue
		}
		md := NewMethodDescriptor(path, m)
		methods[md.String()] = md
	}

	for _, m := range methods {
		for k, v := range m.IncludedStructs {
			// only record names
			m.IncludedStructs[k] = nil
			usedStructs[k] = NewStructDescriptor(k, v)
		}
	}

	resp = &QueryIDLResponse{ServiceInfo: sd}

	return
}

// addType dfs search struct type and put them into type map
func addType(typeMap map[string]*parser.Type, typeStruct *parser.Type) {
	if typeStruct == nil {
		return
	}
	typeName := typeStruct.Name
	if typeName == "list" || typeName == "set" || typeName == "map" {
		kType := typeStruct.KeyType
		vType := typeStruct.ValueType
		addType(typeMap, kType)
		addType(typeMap, vType)
		return
	}
	if typeStruct.Category == parser.Category_Struct {
		typeMap[typeName] = typeStruct
	}
}

func typeToStructLike(path string, typeMap map[string]*parser.Type) map[string]*parser.StructLike {
	realTypeMap := map[string]*parser.Type{}
	// typedef to type
	for structName, typeStruct := range typeMap {
		if !typeStruct.GetIsTypedef() {
			realTypeMap[structName] = typeStruct
			continue
		}
		fromAnother, prefix, rawName := parseName(structName)
		currentPath := path
		if fromAnother {
			currentPath = prefixToPath(path, prefix)
		}
		td, ok := FindTypedef(currentPath, rawName)
		if !ok {
			continue
		}
		tdType := td.Type
		if fromAnother {
			tdType = &parser.Type{
				Name:     prefix + "." + td.Type.Name,
				Category: 13,
			}
		}
		realTypeMap[tdType.Name] = tdType
	}
	structMap := map[string]*parser.StructLike{}
	for structName := range realTypeMap {
		fromAnother, prefix, rawName := parseName(structName)
		currentPath := path
		if fromAnother {
			currentPath = prefixToPath(path, prefix)
		}
		st, ok := FindStructLike(currentPath, rawName)
		if ok {
			structMap[currentPath+"#"+st.Name] = st
		}
	}
	return structMap
}

func dfsStructMap(in map[string]*parser.StructLike) map[string]*parser.StructLike {
	out := map[string]*parser.StructLike{}

	for sname, s := range in {
		out[sname] = s
		structPath := strings.Split(sname, "#")[0]
		typeMap := map[string]*parser.Type{}
		for _, t := range s.Fields {
			addType(typeMap, t.Type)
		}
		innerStructMap := typeToStructLike(structPath, typeMap)
		for k, v := range innerStructMap {
			out[k] = v
		}
	}

	// finish dfs when there's no new struct find from fields
	if len(out) == len(in) {
		return out
	}

	return dfsStructMap(out)
}
