package ttstream

import (
	"context"
	"errors"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
	"github.com/cloudwego/kitex/streamx"
	"github.com/cloudwego/netpoll"
)

func TestTransport(t *testing.T) {
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

	var connDone int32
	var streamDone int32
	svr, err := netpoll.NewEventLoop(nil, netpoll.WithOnConnect(func(ctx context.Context, connection netpoll.Connection) context.Context {
		t.Logf("OnConnect started")
		defer t.Logf("OnConnect finished")
		trans := newTransport(serverTransport, sinfo, connection)
		t.Logf("OnRead started")
		defer t.Log("OnRead finished")

		go func() {
			for {
				s, err := trans.readStream()
				t.Logf("OnRead read stream: %v, %v", s, err)
				if err != nil {
					if err == io.EOF {
						return
					}
					t.Error(err)
				}
				ss := newServerStream(s)
				go func(st streamx.ServerStream) {
					defer func() {
						// set trailer
						err := st.(ServerStreamMeta).SetTrailer(Trailer{"key": "val"})
						test.Assert(t, err == nil, err)

						// send trailer
						err = ss.(*serverStream).sendTrailer()
						test.Assert(t, err == nil, err)
						atomic.AddInt32(&streamDone, -1)
					}()

					// send header
					err := st.(ServerStreamMeta).SendHeader(Header{"key": "val"})
					test.Assert(t, err == nil, err)

					// send data
					for {
						req := new(TestRequest)
						err := st.RecvMsg(ctx, req)
						if errors.Is(err, io.EOF) {
							t.Logf("server stream eof")
							return
						}
						test.Assert(t, err == nil, err)
						t.Logf("server recv msg: %v", req)

						res := req
						err = st.SendMsg(ctx, res)
						if errors.Is(err, io.EOF) {
							return
						}
						test.Assert(t, err == nil, err)
						t.Logf("server send msg: %v", res)
					}
				}(ss)
			}
		}()

		return context.WithValue(ctx, "trans", trans)
	}), netpoll.WithOnDisconnect(func(ctx context.Context, connection netpoll.Connection) {
		t.Logf("OnDisconnect started")
		defer t.Logf("OnDisconnect finished")

		atomic.AddInt32(&connDone, -1)
	}))
	go func() {
		err = svr.Serve(ln)
		test.Assert(t, err == nil, err)
	}()
	test.WaitServerStart(addr)

	// Client
	ctx := context.Background()
	atomic.AddInt32(&connDone, 1)
	conn, err := netpoll.DialConnection("tcp", addr, time.Second)
	test.Assert(t, err == nil, err)
	trans := newTransport(clientTransport, sinfo, conn)

	var wg sync.WaitGroup
	for sid := 1; sid <= 1; sid++ {
		wg.Add(1)
		atomic.AddInt32(&streamDone, 1)
		go func(sid int) {
			defer wg.Done()

			// send header
			s, err := trans.newStream(ctx, method, map[string]string{})
			test.Assert(t, err == nil, err)

			cs := newClientStream(s)
			t.Logf("client stream[%d] created", sid)

			// recv header
			hd, err := cs.Header()
			test.Assert(t, err == nil, err)
			test.Assert(t, hd["key"] == "val", hd)
			t.Logf("client stream[%d] recv header=%v", sid, hd)

			// send and recv data
			for i := 0; i < 3; i++ {
				req := new(TestRequest)
				req.A = 12345
				req.B = "hello"
				res := new(TestResponse)
				err = cs.SendMsg(ctx, req)
				t.Logf("client stream[%d] send msg: %v", sid, req)
				test.Assert(t, err == nil, err)
				err = cs.RecvMsg(ctx, res)
				t.Logf("client stream[%d] recv msg: %v", sid, res)
				test.Assert(t, err == nil, err)
				test.Assert(t, req.A == res.A, res)
				test.Assert(t, req.B == res.B, res)
			}

			// send trailer(trailer is stored in ctx)
			err = cs.CloseSend(ctx)
			test.Assert(t, err == nil, err)
			t.Logf("client stream[%d] close send", sid)

			// recv trailer
			tl, err := cs.Trailer()
			test.Assert(t, err == nil, err)
			test.Assert(t, tl["key"] == "val", tl)
			t.Logf("client stream[%d] recv trailer=%v", sid, tl)
		}(sid)
	}
	wg.Wait()
	for atomic.LoadInt32(&streamDone) != 0 {
		t.Logf("wait all streams closed")
		time.Sleep(time.Millisecond * 10)
	}

	// close conn
	err = trans.close()
	test.Assert(t, err == nil, err)
	err = ln.Close()
	test.Assert(t, err == nil, err)
	for atomic.LoadInt32(&connDone) != 0 {
		time.Sleep(time.Millisecond * 10)
		t.Logf("wait all connections closed")
	}
}
