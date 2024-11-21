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

package ttstream

import (
	"bytes"
	"context"
	"reflect"
	"testing"

	"github.com/cloudwego/gopkg/bufiox"
	"github.com/tidwall/gjson"

	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/generic"
)

func TestFrameCodec(t *testing.T) {
	var buf bytes.Buffer
	writer := bufiox.NewDefaultWriter(&buf)
	reader := bufiox.NewDefaultReader(&buf)
	wframe := newFrame(streamFrame{
		sid:    0,
		method: "method",
		header: map[string]string{"key": "value"},
	}, headerFrameType, []byte("hello world"))

	for i := 0; i < 10; i++ {
		wframe.sid = int32(i)
		err := EncodeFrame(context.Background(), writer, wframe)
		test.Assert(t, err == nil, err)
	}
	err := writer.Flush()
	test.Assert(t, err == nil, err)

	for i := 0; i < 10; i++ {
		rframe, err := DecodeFrame(context.Background(), reader)
		test.Assert(t, err == nil, err)
		test.DeepEqual(t, string(wframe.payload), string(rframe.payload))
		test.DeepEqual(t, wframe.header, rframe.header)
	}
}

func TestFrameWithoutPayloadCodec(t *testing.T) {
	rmsg := new(testRequest)
	rmsg.A = 1
	payload, err := EncodePayload(context.Background(), rmsg, "")
	test.Assert(t, err == nil, err)

	wmsg := new(testRequest)
	err = DecodePayload(context.Background(), payload, wmsg, "")
	test.Assert(t, err == nil, err)
	test.DeepEqual(t, wmsg, rmsg)
}

func TestPayloadCodec(t *testing.T) {
	rmsg := new(testRequest)
	rmsg.A = 1
	rmsg.B = "hello world"
	payload, err := EncodePayload(context.Background(), rmsg, "")
	test.Assert(t, err == nil, err)

	wmsg := new(testRequest)
	err = DecodePayload(context.Background(), payload, wmsg, "")
	test.Assert(t, err == nil, err)
	test.DeepEqual(t, wmsg, rmsg)
}

func TestGenericPayloadCodec(t *testing.T) {
	idlContent := `
		struct testRequest {
			1: i32 A,
			2: string B,
		}
		
		service Test {
			testRequest Bidi(1: testRequest request) (streaming.mode="bidirectional"),
		}
	`
	p, err := generic.NewThriftContentProvider(idlContent, nil)
	test.Assert(t, err == nil, err)
	g, err := generic.JSONThriftGeneric(p)

	req := `{"A":1, "B":"hello world"}`
	args := &generic.Args{
		Request: req,
		Method:  "Bidi",
	}
	args.SetCodec(g.MessageReaderWriter())
	payload, err := EncodePayload(context.Background(), args, "Bidi")
	test.Assert(t, err == nil, err)

	result := &generic.Result{}
	result.SetCodec(g.MessageReaderWriter())
	err = DecodePayload(context.Background(), payload, result, "Bidi")
	test.Assert(t, err == nil, err)
	resp, ok := result.Success.(string)
	test.Assert(t, ok)
	test.Assert(t, reflect.DeepEqual(gjson.Get(resp, "A").String(), "1"))
	test.Assert(t, reflect.DeepEqual(gjson.Get(resp, "B").String(), "hello world"))
}
