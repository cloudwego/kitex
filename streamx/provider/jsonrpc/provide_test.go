package jsonrpc

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
	"io"
	"net"
	"sync"
	"testing"
	"time"
)

func TestCodec(t *testing.T) {
	var buf bytes.Buffer
	writer := bufio.NewWriter(&buf)
	f1 := newFrame(0, 1, "test", []byte("12345"))
	err := EncodeFrame(writer, f1)
	test.Assert(t, err == nil, err)
	_ = writer.Flush()
	reader := bufio.NewReader(&buf)
	f2, err := DecodeFrame(reader)
	test.Assert(t, err == nil, err)
	test.Assert(t, f2.method == f1.method, f2.method)
	test.Assert(t, string(f2.payload) == string(f2.payload), f2.payload)
}

func TestClientAndServer(t *testing.T) {
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
				func() interface{} { return nil },
				func() interface{} { return nil },
				false,
				serviceinfo.WithStreamingMode(serviceinfo.StreamingBidirectional),
			),
		},
		Extra: map[string]interface{}{"streaming": true},
	}

	addr := "127.0.0.1:12345"
	ln, err := net.Listen("tcp", addr)
	test.Assert(t, err == nil, err)

	var wg sync.WaitGroup
	// Server
	wg.Add(2)
	go func() {
		defer wg.Done()
		for {
			conn, err := ln.Accept()
			if conn == nil && err == nil || errors.Is(err, net.ErrClosed) {
				return
			}
			test.Assert(t, err == nil, err)
			go func() {
				defer wg.Done()
				server := newTransport(sinfo, conn)
				st, nerr := server.readStream()
				if nerr != nil {
					if nerr != io.EOF {
						return
					}
					panic(err)
				}
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
						if nerr != io.EOF {
							return
						}
						panic(err)
					}
				}
			}()
		}
	}()
	time.Sleep(time.Millisecond * 100)

	// Client
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

	err = cs.CloseSend(ctx)
	test.Assert(t, err == nil, err)
	err = ln.Close()
	test.Assert(t, err == nil, err)
	err = trans.close()
	test.Assert(t, err == nil, err)
	wg.Wait()
}
