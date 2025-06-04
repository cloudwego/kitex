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

package generic

import (
	"context"
	"testing"

	"github.com/cloudwego/gopkg/bufiox"

	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/generic/thrift"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
)

func TestMapThriftCodec(t *testing.T) {
	p, err := NewThriftFileProvider("./map_test/idl/mock.thrift")
	test.Assert(t, err == nil)
	mtc := newMapThriftCodec(p, false)
	defer mtc.Close()
	test.Assert(t, mtc.Name() == "MapThrift")

	method, err := mtc.getMethod(nil, "Test")
	test.Assert(t, err == nil)
	test.Assert(t, method.Name == "Test")
	test.Assert(t, method.StreamingMode == serviceinfo.StreamingNone)
	test.Assert(t, mtc.svcName == "Mock")
	test.Assert(t, mtc.extra[CombineServiceKey] == "false")

	rw := mtc.getMessageReaderWriter()

	var buf []byte
	out := bufiox.NewBytesWriter(&buf)

	err = rw.(*thrift.StructReaderWriter).Write(context.Background(), out, map[string]interface{}{
		"strMap": map[string]interface{}{"hello": "world"},
	}, "Test", true, nil)
	test.Assert(t, err == nil)

	err = out.Flush()
	test.Assert(t, err == nil)

	in := bufiox.NewBytesReader(buf)

	res, err := rw.(*thrift.StructReaderWriter).Read(context.Background(), "Test", false, len(buf), in)
	test.Assert(t, err == nil)

	test.Assert(t, res.(map[string]interface{})["strMap"].(map[interface{}]interface{})["hello"].(string) == "world")
}

func TestMapThriftCodecSelfRef(t *testing.T) {
	p, err := NewThriftFileProvider("./map_test/idl/self_ref.thrift")
	test.Assert(t, err == nil)
	mtc := newMapThriftCodec(p, false)
	defer mtc.Close()
	test.Assert(t, mtc.Name() == "MapThrift")

	method, err := mtc.getMethod(nil, "Test")
	test.Assert(t, err == nil)
	test.Assert(t, method.Name == "Test")
	test.Assert(t, method.StreamingMode == serviceinfo.StreamingNone)
	test.Assert(t, mtc.svcName == "Mock")

	rw := mtc.getMessageReaderWriter()
	_, ok := rw.(*thrift.StructReaderWriter)
	test.Assert(t, ok)
}

func TestMapThriftCodecForJSON(t *testing.T) {
	p, err := NewThriftFileProvider("./map_test/idl/mock.thrift")
	test.Assert(t, err == nil)
	mtc := newMapThriftCodec(p, true)
	defer mtc.Close()
	test.Assert(t, mtc.Name() == "MapThrift")

	method, err := mtc.getMethod(nil, "Test")
	test.Assert(t, err == nil)
	test.Assert(t, method.Name == "Test")
	test.Assert(t, method.StreamingMode == serviceinfo.StreamingNone)
	test.Assert(t, mtc.svcName == "Mock")

	rw := mtc.getMessageReaderWriter()

	var buf []byte
	out := bufiox.NewBytesWriter(&buf)

	err = rw.(*thrift.StructReaderWriter).Write(context.Background(), out, map[string]interface{}{
		"strMap": map[string]interface{}{"hello": "world"},
	}, "Test", true, nil)
	test.Assert(t, err == nil)

	err = out.Flush()
	test.Assert(t, err == nil)

	in := bufiox.NewBytesReader(buf)

	res, err := rw.(*thrift.StructReaderWriter).Read(context.Background(), "Test", false, len(buf), in)
	test.Assert(t, err == nil)

	test.Assert(t, res.(map[string]interface{})["strMap"].(map[string]interface{})["hello"].(string) == "world")
}
