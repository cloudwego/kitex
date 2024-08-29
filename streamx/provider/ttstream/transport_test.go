package ttstream

import (
	"context"
	"errors"
	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
	"github.com/cloudwego/netpoll"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"
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

	addr := "127.0.0.1:12345"
	ln, err := net.Listen("tcp", addr)
	test.Assert(t, err == nil, err)

	var connDone int32
	var streamDone int32
	svr, err := netpoll.NewEventLoop(nil, netpoll.WithOnConnect(func(ctx context.Context, connection netpoll.Connection) context.Context {
		t.Logf("OnConnect started")
		defer t.Logf("OnConnect finished")
		trans := newTransport(sinfo, connection)
		t.Logf("OnRead started")
		defer t.Log("OnRead finished")

		go func() {
			for {
				st, err := trans.readStream()
				t.Logf("OnRead read stream: %v, %v", st, err)
				if err != nil {
					if err == io.EOF {
						return
					}
					t.Error(err)
				}
				go func(st *stream) {
					defer atomic.AddInt32(&streamDone, -1)
					for {
						req := new(TestRequest)
						err := st.RecvMsg(ctx, req)
						if errors.Is(err, io.EOF) {
							t.Logf("server stream eof: %d", st.sid)
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
				}(st)
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
	time.Sleep(time.Millisecond * 100)

	// Client
	ctx := context.Background()
	atomic.AddInt32(&connDone, 1)
	conn, err := netpoll.DialConnection("tcp", addr, time.Second)
	test.Assert(t, err == nil, err)
	trans := newTransport(sinfo, conn)

	var wg sync.WaitGroup
	for sid := 1; sid <= 3; sid++ {
		wg.Add(1)
		atomic.AddInt32(&streamDone, 1)
		go func() {
			defer wg.Done()
			s, err := trans.newStream(ctx, method)
			test.Assert(t, err == nil, err)
			cs := newClientStream(s)

			for i := 0; i < 3; i++ {
				req := new(TestRequest)
				req.A = 12345
				req.B = "hello"
				res := new(TestResponse)
				err = cs.SendMsg(ctx, req)
				t.Logf("client send msg: %v", req)
				test.Assert(t, err == nil, err)
				err = cs.RecvMsg(ctx, res)
				t.Logf("client recv msg: %v", res)
				test.Assert(t, err == nil, err)
				test.Assert(t, req.A == res.A, res)
				test.Assert(t, req.B == res.B, res)
			}

			// close stream
			err = cs.CloseSend(ctx)
			test.Assert(t, err == nil, err)
		}()
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
