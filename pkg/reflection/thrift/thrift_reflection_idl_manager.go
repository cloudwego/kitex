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
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/cloudwego/thriftgo/parser"
	"github.com/cloudwego/thriftgo/reflection"
)

var IDLManager = map[string]*reflection.FileDescriptor{}

func RegisterIDL(bytes []byte) {
	AST := reflection.Decode(bytes)
	// TODO: filenames conflict?
	IDLManager[AST.Filename] = AST
}

func prefixToPath(currentPath, prefix string) string {
	im := IDLManager[currentPath].IncludeMap
	includePath := im[prefix]
	return filepath.Join(filepath.Dir(currentPath), includePath)
}

func FindTypedef(path, typedefName string) (*parser.Typedef, bool) {
	currentAST := IDLManager[path]
	for _, td := range currentAST.Typedefs {
		if td.Alias == typedefName {
			return td, true
		}
	}
	return nil, false
}

func FindStructLike(path, structName string) (*parser.StructLike, bool) {
	currentAST := IDLManager[path]
	for _, st := range currentAST.Structs {
		if st.Name == structName {
			return st, true
		}
	}
	return nil, false
}

type MethodDescriptor struct {
	MethodIDLPath   string                        `json:"MethodIDLPath"`
	MethodName      string                        `json:"MethodName"`
	MethodSignature string                        `json:"MethodSignature"`
	IncludedStructs map[string]*parser.StructLike `json:"IncludedStructs"`
}

func (md *MethodDescriptor) MarshalJSON() ([]byte, error) {
	buf := bytes.NewBuffer(nil)
	buf.WriteString("{\"MethodIDLPath\":\"")
	buf.WriteString(md.MethodIDLPath)
	buf.WriteString("\",\"MethodName\":\"")
	buf.WriteString(md.MethodName)
	buf.WriteString("\",\"MethodSignature\":\"")
	buf.WriteString(md.MethodSignature)
	buf.WriteString("\",\"IncludedStructs\":{")
	snames := make([]string, 0, len(md.IncludedStructs))
	for n := range md.IncludedStructs {
		snames = append(snames, n)
	}
	sort.Slice(snames, func(i, j int) bool {
		return snames[i] < snames[j]
	})
	for i, sn := range snames {
		buf.WriteString("\"" + sn + "\":")
		sbuf, _ := json.Marshal(md.IncludedStructs[sn])
		buf.Write(sbuf)
		if i < len(snames)-1 {
			buf.WriteByte(',')
		}
	}
	buf.WriteString("}}")
	return buf.Bytes(), nil
}

type StructDescriptor struct {
	StructIDLPath   string `json:"StructIDLPath"`
	StructName      string `json:"StructName"`
	StructSignature string `json:"StructSignature"`
}

type ServiceDescriptor struct {
	Methods     map[string]*MethodDescriptor `json:"Methods"`
	UsedStructs map[string]*StructDescriptor `json:"UsedStructs"`
}

func (sd *ServiceDescriptor) MarshalJSON() ([]byte, error) {
	buf := bytes.NewBuffer(nil)
	buf.WriteString(`{"Methods":{`)
	methods := make([]string, 0, len(sd.Methods))
	for m := range sd.Methods {
		methods = append(methods, m)
	}
	sort.Slice(methods, func(i, j int) bool {
		return methods[i] < methods[j]
	})
	for i, method := range methods {
		buf.WriteString(`"` + method + `":`)
		mdbuf, _ := sd.Methods[method].MarshalJSON()
		buf.Write(mdbuf)
		if i < len(methods)-1 {
			buf.WriteByte(',')
		}
	}
	buf.WriteString(`},"UsedStructs":{`)
	structNames := make([]string, 0, len(sd.UsedStructs))
	for sn := range sd.UsedStructs {
		structNames = append(structNames, sn)
	}
	sort.Slice(structNames, func(i, j int) bool {
		return structNames[i] < structNames[j]
	})
	for i, sn := range structNames {
		buf.WriteString(`"` + sn + `":`)
		sdbuf, _ := json.Marshal(sd.UsedStructs[sn])
		buf.Write(sdbuf)
		if i < len(structNames)-1 {
			buf.WriteByte(',')
		}
	}
	buf.WriteString(`}}`)
	return buf.Bytes(), nil
}

func NewMethodDescriptor(path string, m *parser.Function) *MethodDescriptor {
	argsArr := []string{}
	typeMap := map[string]*parser.Type{}
	for _, arg := range m.Arguments {
		argDesc := fmt.Sprintf("%d:%s %s %s", arg.ID, arg.Requiredness.String(), arg.Type.String(), arg.Name)
		argsArr = append(argsArr, argDesc)
		addType(typeMap, arg.Type)
	}
	retType := m.FunctionType.String()
	addType(typeMap, m.FunctionType)
	methodSignature := fmt.Sprintf("%s %s(%s)", retType, m.Name, strings.Join(argsArr, ","))

	// types and typedef -> structLike
	structMap := typeToStructLike(path, typeMap)
	// dfs find all included struct
	structMap = dfsStructMap(structMap)

	return &MethodDescriptor{
		MethodIDLPath:   path,
		MethodName:      m.Name,
		MethodSignature: methodSignature,
		IncludedStructs: structMap,
	}
}

func NewStructDescriptor(name string, structLike *parser.StructLike) *StructDescriptor {
	fieldDescs := []string{}
	for _, f := range structLike.Fields {
		desc := fmt.Sprintf("  %d:%s %s %s", f.GetID(), f.Requiredness.String(), f.Type.String(), f.GetName())
		fieldDescs = append(fieldDescs, desc)
	}
	signature := fmt.Sprintf("type %s struct{\n%s\n}", structLike.Name, strings.Join(fieldDescs, "\n"))

	return &StructDescriptor{
		StructIDLPath:   strings.TrimSuffix(name, "#"+structLike.Name),
		StructName:      structLike.Name,
		StructSignature: signature,
	}
}

func (md *MethodDescriptor) String() string {
	return md.MethodIDLPath + "#" + md.MethodName
}

func findSvcFromAll(svcName string) (string, *parser.Service, bool) {
	for path := range IDLManager {
		svc, ok := findSvc(path, svcName)
		if ok {
			return path, svc, true
		}
	}
	return "", nil, false
}

func findMethodFromAll(methodName string) (string, *parser.Function, bool) {
	for path, ast := range IDLManager {
		for _, svc := range ast.Services {
			for _, m := range svc.Functions {
				if m.Name == methodName {
					return path, m, true
				}
			}
		}
	}
	return "", nil, false
}

func findSvc(prefix, svcName string) (*parser.Service, bool) {
	currentAST := IDLManager[prefix]
	if currentAST != nil {
		for _, s := range currentAST.Services {
			if s.Name == svcName {
				return s, true
			}
		}
	}
	return nil, false
}

func parseName(structName string) (hasPrefix bool, pkgPrefix, realStructName string) {
	if strings.Contains(structName, ".") {
		arr := strings.Split(structName, ".")
		realStructName = arr[len(arr)-1]
		pkgPrefix = strings.TrimSuffix(structName, "."+realStructName)
		hasPrefix = true
		return
	}
	return false, "", structName
}