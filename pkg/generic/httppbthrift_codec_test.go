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

package generic

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"reflect"
	"testing"

	"github.com/bytedance/mockey"

	"github.com/cloudwego/kitex/internal/test"
	gthrift "github.com/cloudwego/kitex/pkg/generic/thrift"
)

// TestFromHTTPPbRequest this test is to make sure mockey can work.
// If it fails, please run this test with `-gcflags="all=-N -l"`
func TestFromHTTPPbRequest(t *testing.T) {
	mockey.PatchConvey("TestFromHTTPPbRequest", t, func() {
		req, err := http.NewRequest("POST", "/far/boo", bytes.NewBuffer([]byte("321")))
		test.Assert(t, err == nil)
		mockey.Mock(io.ReadAll).Return([]byte("123"), nil).Build()
		hreq, err := FromHTTPPbRequest(req)
		test.Assert(t, err == nil)
		test.Assert(t, reflect.DeepEqual(hreq.RawBody, []byte("123")), string(hreq.RawBody))
		test.Assert(t, hreq.GetMethod() == "POST")
		test.Assert(t, hreq.GetPath() == "/far/boo")
	})
}

func TestHTTPPbThriftCodec(t *testing.T) {
	p, err := NewThriftFileProvider("./httppb_test/idl/echo.thrift")
	test.Assert(t, err == nil)
	pbIdl := "./httppb_test/idl/echo.proto"
	pbf, err := os.Open(pbIdl)
	test.Assert(t, err == nil)
	pbContent, err := io.ReadAll(pbf)
	test.Assert(t, err == nil)
	pbf.Close()
	pbp, err := NewPbContentProvider(pbIdl, map[string]string{pbIdl: string(pbContent)})
	test.Assert(t, err == nil)

	htc := newHTTPPbThriftCodec(p, pbp)
	defer htc.Close()
	test.Assert(t, htc.Name() == "HttpPbThrift")

	req, err := http.NewRequest("GET", "/Echo", bytes.NewBuffer([]byte("321")))
	test.Assert(t, err == nil)
	hreq, err := FromHTTPPbRequest(req)
	test.Assert(t, err == nil)
	// wrong
	method, err := htc.getMethod("test")
	test.Assert(t, err.Error() == "req is invalid, need descriptor.HTTPRequest" && method == nil)
	// right
	method, err = htc.getMethod(hreq)
	test.Assert(t, err == nil, err)
	test.Assert(t, method.Name == "Echo")
	test.Assert(t, htc.svcName == "ExampleService")

	rw := htc.getMessageReaderWriter()
	_, ok := rw.(gthrift.MessageWriter)
	test.Assert(t, ok)
	_, ok = rw.(gthrift.MessageReader)
	test.Assert(t, ok)
}
