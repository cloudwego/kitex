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

package jsonrpc

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
)

func TestCodec(t *testing.T) {
	var buf bytes.Buffer
	writer := bufio.NewWriter(&buf)
	f1 := newFrame(0, 1, "a.b.c", "test", []byte("12345"))
	err := EncodeFrame(writer, f1)
	test.Assert(t, err == nil, err)
	_ = writer.Flush()
	reader := bufio.NewReader(&buf)
	f2, err := DecodeFrame(reader)
	test.Assert(t, err == nil, err)
	test.Assert(t, f2.method == f1.method, f2.method)
	test.Assert(t, string(f2.payload) == string(f1.payload), f2.payload)
}

func TestTransport(t *testing.T) {
	type TestRequest struct {
		A int    `json:"A,omitempty"`
		B string `json:"B,omitempty"`
	}
	type TestResponse = TestRequest
	method := "BidiStream"
	sinfo := &serviceinfo.ServiceInfo{
		ServiceName: "a.b.c",
		Methods: map[string]serviceinfo.MethodInfo{
			method: serviceinfo.NewMethodInfo(
				func(ctx context.Context, handler, reqArgs, resArgs interface{}) error {
					return nil
				},
				nil,
				nil,
				false,
				serviceinfo.WithStreamingMode(serviceinfo.StreamingBidirectional),
			),
		},
		Extra: map[string]interface{}{"streaming": true},
	}

	addr := test.GetLocalAddress()
	ln, err := net.Listen("tcp", addr)
	test.Assert(t, err == nil, err)

	// Server
	var connDone int32
	var streamDone int32
	go func() {
		for {
			conn, err := ln.Accept()
			if (conn == nil && err == nil) || errors.Is(err, net.ErrClosed) {
				return
			}
			test.Assert(t, err == nil, err)
			go func() {
				defer atomic.AddInt32(&connDone, -1)
				server := newTransport(sinfo, conn)
				st, nerr := server.readStream()
				if nerr != nil {
					if nerr == io.EOF {
						return
					}
					t.Error(nerr)
				}
				go func() {
					defer atomic.AddInt32(&streamDone, -1)
					for {
						ctx := context.Background()
						req := new(TestRequest)
						nerr = st.RecvMsg(ctx, req)
						if errors.Is(nerr, io.EOF) {
							return
						}
						t.Logf("server recv msg: %v %v", req, nerr)
						res := req
						nerr = st.SendMsg(ctx, res)
						t.Logf("server send msg: %v %v", res, nerr)
						if nerr != nil {
							if nerr == io.EOF {
								return
							}
							t.Error(nerr)
						}
					}
				}()
			}()
		}
	}()
	time.Sleep(time.Millisecond * 100)

	// Client
	atomic.AddInt32(&connDone, 1)
	conn, err := net.Dial("tcp", addr)
	test.Assert(t, err == nil, err)
	trans := newTransport(sinfo, conn)
	s, err := trans.newStream(method)
	test.Assert(t, err == nil, err)
	cs := newClientStream(s)

	req := new(TestRequest)
	req.A = 12345
	req.B = "hello"
	res := new(TestResponse)
	ctx := context.Background()
	err = cs.SendMsg(ctx, req)
	t.Logf("client send msg: %v", req)
	test.Assert(t, err == nil, err)
	err = cs.RecvMsg(ctx, res)
	t.Logf("client recv msg: %v", res)
	test.Assert(t, err == nil, err)
	test.Assert(t, req.A == res.A, res)
	test.Assert(t, req.B == res.B, res)

	// close stream
	err = cs.CloseSend(ctx)
	test.Assert(t, err == nil, err)
	for atomic.LoadInt32(&streamDone) != 0 {
		time.Sleep(time.Millisecond * 10)
	}

	// close conn
	err = trans.close()
	test.Assert(t, err == nil, err)
	err = ln.Close()
	test.Assert(t, err == nil, err)
	for atomic.LoadInt32(&connDone) != 0 {
		time.Sleep(time.Millisecond * 10)
	}
}
