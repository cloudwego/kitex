//go:build !windows

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

package ttstream

import (
	"context"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/cloudwego/netpoll"

	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/streaming"
)

func TestStreamTimeout(t *testing.T) {
	cliSvcName := "clientSideService"
	t.Run("ServerStreaming", func(t *testing.T) {
		method := "ServerStreaming"
		t.Run("configure timeout and finish in time", func(t *testing.T) {
			cfd, sfd := netpoll.GetSysFdPairs()
			cconn, err := netpoll.NewFDConnection(cfd)
			test.Assert(t, err == nil, err)
			sconn, err := netpoll.NewFDConnection(sfd)
			test.Assert(t, err == nil, err)

			intHeader := make(IntHeader)
			strHeader := make(streaming.Header)
			ctrans := newClientTransport(cconn, nil)
			defer func() {
				ctrans.Close(nil)
				ctrans.WaitClosed()
			}()
			start := time.Now()
			tm := 200 * time.Millisecond
			ctx, cancel := context.WithTimeout(context.Background(), tm)
			defer cancel()
			injectStreamTimeout(ctx, intHeader)
			cs := newClientStream(ctx, ctrans, streamFrame{sid: genStreamID(), method: method})
			cs.rpcInfo = rpcinfo.NewRPCInfo(
				rpcinfo.NewEndpointInfo(cliSvcName, method, nil, nil), nil, nil, nil, nil)
			cs.setStreamTimeout(tm)
			err = ctrans.WriteStream(ctx, cs, intHeader, strHeader)
			test.Assert(t, err == nil, err)
			strans := newServerTransport(sconn)
			defer func() {
				strans.Close(nil)
				strans.WaitClosed()
			}()
			ss, err := strans.ReadStream(context.Background())
			test.Assert(t, err == nil, err)

			var wg sync.WaitGroup
			wg.Add(1)
			// client
			go func() {
				defer wg.Done()
				req := new(testRequest)
				req.B = "hello"
				cErr := cs.SendMsg(ctx, req)
				test.Assert(t, cErr == nil, cErr)

				cErr = cs.CloseSend(ctx)
				test.Assert(t, cErr == nil, cErr)

				for {
					res := new(testResponse)
					cErr = cs.RecvMsg(ctx, res)
					if cErr == io.EOF {
						break
					}
					test.Assert(t, cErr == nil, cErr)
				}
			}()

			// server
			checkServerSideStreamTimeout(t, ss, start, tm)

			req := new(testRequest)
			sErr := ss.RecvMsg(ss.ctx, req)
			test.Assert(t, sErr == nil, sErr)

			for i := 0; i < 10; i++ {
				res := new(testResponse)
				res.B = req.B
				sErr = ss.SendMsg(ss.ctx, res)
				test.Assert(t, sErr == nil, sErr)
			}
			err = ss.CloseSend(nil)
			test.Assert(t, err == nil, err)
			wg.Wait()
		})
		t.Run("configure timeout and server-side continue sending", func(t *testing.T) {
			cfd, sfd := netpoll.GetSysFdPairs()
			cconn, err := netpoll.NewFDConnection(cfd)
			test.Assert(t, err == nil, err)
			sconn, err := netpoll.NewFDConnection(sfd)
			test.Assert(t, err == nil, err)

			intHeader := make(IntHeader)
			strHeader := make(streaming.Header)
			ctrans := newClientTransport(cconn, nil)
			defer func() {
				ctrans.Close(nil)
				ctrans.WaitClosed()
			}()
			start := time.Now()
			tm := 50 * time.Millisecond
			ctx, cancel := context.WithTimeout(context.Background(), tm)
			defer cancel()
			injectStreamTimeout(ctx, intHeader)
			cs := newClientStream(ctx, ctrans, streamFrame{sid: genStreamID(), method: method})
			cs.rpcInfo = rpcinfo.NewRPCInfo(
				rpcinfo.NewEndpointInfo(cliSvcName, method, nil, nil), nil, nil, nil, nil)
			cs.setStreamTimeout(tm)
			err = ctrans.WriteStream(ctx, cs, intHeader, strHeader)
			test.Assert(t, err == nil, err)
			strans := newServerTransport(sconn)
			defer func() {
				strans.Close(nil)
				strans.WaitClosed()
			}()
			ss, err := strans.ReadStream(context.Background())
			test.Assert(t, err == nil, err)

			var wg sync.WaitGroup
			wg.Add(1)
			// client
			go func() {
				defer wg.Done()
				req := new(testRequest)
				req.B = "hello"
				cErr := cs.SendMsg(ctx, req)
				test.Assert(t, cErr == nil, cErr)

				cErr = cs.CloseSend(ctx)
				test.Assert(t, cErr == nil, cErr)

				// Try to receive, should timeout eventually
				for {
					res := new(testResponse)
					cErr = cs.RecvMsg(ctx, res)
					if cErr != nil {
						t.Logf("client-side Stream Recv err: %v", cErr)
						test.Assert(t, errors.Is(cErr, kerrors.ErrStreamingTimeout), cErr)
						break
					}
				}
			}()

			// server
			checkServerSideStreamTimeout(t, ss, start, tm)

			req := new(testRequest)
			sErr := ss.RecvMsg(ss.ctx, req)
			test.Assert(t, sErr == nil, sErr)

			// Server keeps sending slowly, causing timeout
			for {
				res := new(testResponse)
				res.B = req.B
				sErr = ss.SendMsg(ss.ctx, res)
				if sErr != nil {
					t.Logf("server-side Stream Send err: %v", sErr)
					test.Assert(t, errors.Is(sErr, kerrors.ErrStreamingTimeout), sErr)
					break
				}
				time.Sleep(30 * time.Millisecond)
			}
			wg.Wait()
		})
		t.Run("configure timeout and server-side spend a lot of time send the first resp", func(t *testing.T) {
			cfd, sfd := netpoll.GetSysFdPairs()
			cconn, err := netpoll.NewFDConnection(cfd)
			test.Assert(t, err == nil, err)
			sconn, err := netpoll.NewFDConnection(sfd)
			test.Assert(t, err == nil, err)

			intHeader := make(IntHeader)
			strHeader := make(streaming.Header)
			ctrans := newClientTransport(cconn, nil)
			defer func() {
				ctrans.Close(nil)
				ctrans.WaitClosed()
			}()
			start := time.Now()
			tm := 50 * time.Millisecond
			ctx, cancel := context.WithTimeout(context.Background(), tm)
			defer cancel()
			injectStreamTimeout(ctx, intHeader)
			cs := newClientStream(ctx, ctrans, streamFrame{sid: genStreamID(), method: method})
			cs.rpcInfo = rpcinfo.NewRPCInfo(
				rpcinfo.NewEndpointInfo(cliSvcName, method, nil, nil), nil, nil, nil, nil)
			cs.setStreamTimeout(tm)
			err = ctrans.WriteStream(ctx, cs, intHeader, strHeader)
			test.Assert(t, err == nil, err)
			strans := newServerTransport(sconn)
			defer func() {
				strans.Close(nil)
				strans.WaitClosed()
			}()
			ss, err := strans.ReadStream(context.Background())
			test.Assert(t, err == nil, err)

			var wg sync.WaitGroup
			wg.Add(1)
			// client
			go func() {
				defer wg.Done()
				req := new(testRequest)
				req.B = "hello"
				cErr := cs.SendMsg(ctx, req)
				test.Assert(t, cErr == nil, cErr)

				cErr = cs.CloseSend(ctx)
				test.Assert(t, cErr == nil, cErr)

				// Try to receive first response, should timeout
				res := new(testResponse)
				cErr = cs.RecvMsg(ctx, res)
				test.Assert(t, cErr != nil)
				t.Logf("client-side Stream Recv err: %v", cErr)
				test.Assert(t, errors.Is(cErr, kerrors.ErrStreamingTimeout), cErr)
			}()

			// server
			checkServerSideStreamTimeout(t, ss, start, tm)

			req := new(testRequest)
			sErr := ss.RecvMsg(ss.ctx, req)
			test.Assert(t, sErr == nil, sErr)

			// Server delays before sending first response
			time.Sleep(150 * time.Millisecond)

			res := new(testResponse)
			res.B = req.B
			sErr = ss.SendMsg(ss.ctx, res)
			// Send will fail because client already timeout
			test.Assert(t, sErr != nil)
			t.Logf("server-side Stream Send err: %v", sErr)
			test.Assert(t, errors.Is(sErr, kerrors.ErrStreamingTimeout), sErr)
			wg.Wait()
		})
	})
	t.Run("ClientStreaming", func(t *testing.T) {
		method := "ClientStreaming"
		t.Run("configure timeout and finish in time", func(t *testing.T) {
			cfd, sfd := netpoll.GetSysFdPairs()
			cconn, err := netpoll.NewFDConnection(cfd)
			test.Assert(t, err == nil, err)
			sconn, err := netpoll.NewFDConnection(sfd)
			test.Assert(t, err == nil, err)

			intHeader := make(IntHeader)
			strHeader := make(streaming.Header)
			ctrans := newClientTransport(cconn, nil)
			defer func() {
				ctrans.Close(nil)
				ctrans.WaitClosed()
			}()
			start := time.Now()
			tm := 200 * time.Millisecond
			ctx, cancel := context.WithTimeout(context.Background(), tm)
			defer cancel()
			injectStreamTimeout(ctx, intHeader)
			cs := newClientStream(ctx, ctrans, streamFrame{sid: genStreamID(), method: method})
			cs.rpcInfo = rpcinfo.NewRPCInfo(
				rpcinfo.NewEndpointInfo(cliSvcName, method, nil, nil), nil, nil, nil, nil)
			cs.setStreamTimeout(tm)
			err = ctrans.WriteStream(ctx, cs, intHeader, strHeader)
			test.Assert(t, err == nil, err)
			strans := newServerTransport(sconn)
			defer func() {
				strans.Close(nil)
				strans.WaitClosed()
			}()
			ss, err := strans.ReadStream(context.Background())
			test.Assert(t, err == nil, err)

			var wg sync.WaitGroup
			wg.Add(1)
			// client
			go func() {
				defer wg.Done()
				// Send 10 messages quickly
				for i := 0; i < 10; i++ {
					req := new(testRequest)
					req.A = int32(i)
					req.B = "hello"
					cErr := cs.SendMsg(ctx, req)
					test.Assert(t, cErr == nil, cErr)
				}

				cErr := cs.CloseSend(ctx)
				test.Assert(t, cErr == nil, cErr)

				// Receive final response
				res := new(testResponse)
				cErr = cs.RecvMsg(ctx, res)
				test.Assert(t, cErr == nil, cErr)
			}()

			// server
			checkServerSideStreamTimeout(t, ss, start, tm)

			for {
				req := new(testRequest)
				sErr := ss.RecvMsg(ss.ctx, req)
				if sErr == io.EOF {
					break
				}
				test.Assert(t, sErr == nil, sErr)
			}

			// Send final response
			res := new(testResponse)
			res.B = "done"
			sErr := ss.SendMsg(ss.ctx, res)
			test.Assert(t, sErr == nil, sErr)

			err = ss.CloseSend(nil)
			test.Assert(t, err == nil, err)
			wg.Wait()
		})
		t.Run("configure timeout and client-side continue sending", func(t *testing.T) {
			cfd, sfd := netpoll.GetSysFdPairs()
			cconn, err := netpoll.NewFDConnection(cfd)
			test.Assert(t, err == nil, err)
			sconn, err := netpoll.NewFDConnection(sfd)
			test.Assert(t, err == nil, err)

			intHeader := make(IntHeader)
			strHeader := make(streaming.Header)
			ctrans := newClientTransport(cconn, nil)
			defer func() {
				ctrans.Close(nil)
				ctrans.WaitClosed()
			}()
			start := time.Now()
			tm := 50 * time.Millisecond
			ctx, cancel := context.WithTimeout(context.Background(), tm)
			defer cancel()
			injectStreamTimeout(ctx, intHeader)
			cs := newClientStream(ctx, ctrans, streamFrame{sid: genStreamID(), method: method})
			cs.rpcInfo = rpcinfo.NewRPCInfo(
				rpcinfo.NewEndpointInfo(cliSvcName, method, nil, nil), nil, nil, nil, nil)
			cs.setStreamTimeout(tm)
			err = ctrans.WriteStream(ctx, cs, intHeader, strHeader)
			test.Assert(t, err == nil, err)
			strans := newServerTransport(sconn)
			defer func() {
				strans.Close(nil)
				strans.WaitClosed()
			}()
			ss, err := strans.ReadStream(context.Background())
			test.Assert(t, err == nil, err)

			var wg sync.WaitGroup
			wg.Add(1)
			// client
			go func() {
				defer wg.Done()
				for i := 0; i < 10; i++ {
					req := new(testRequest)
					req.A = int32(i)
					req.B = "hello"
					cErr := cs.SendMsg(ctx, req)
					if cErr != nil {
						// Send will fail after timeout
						t.Logf("client-side Stream Send err: %v", cErr)
						test.Assert(t, errors.Is(cErr, kerrors.ErrStreamingTimeout), cErr)
						break
					}
					time.Sleep(30 * time.Millisecond) // Slow sending
				}
			}()

			// server
			checkServerSideStreamTimeout(t, ss, start, tm)

			// Try to receive, should timeout (cascaded from client)
			for {
				req := new(testRequest)
				sErr := ss.RecvMsg(ss.ctx, req)
				if sErr != nil {
					t.Logf("server-side Stream Recv err: %v", sErr)
					test.Assert(t, errors.Is(sErr, kerrors.ErrStreamingTimeout), sErr)
					break
				}
			}
			wg.Wait()
		})
		t.Run("configure timeout and server-side spend a lot of time return the resp", func(t *testing.T) {
			cfd, sfd := netpoll.GetSysFdPairs()
			cconn, err := netpoll.NewFDConnection(cfd)
			test.Assert(t, err == nil, err)
			sconn, err := netpoll.NewFDConnection(sfd)
			test.Assert(t, err == nil, err)

			intHeader := make(IntHeader)
			strHeader := make(streaming.Header)
			ctrans := newClientTransport(cconn, nil)
			defer func() {
				ctrans.Close(nil)
				ctrans.WaitClosed()
			}()
			start := time.Now()
			tm := 50 * time.Millisecond
			ctx, cancel := context.WithTimeout(context.Background(), tm)
			defer cancel()
			injectStreamTimeout(ctx, intHeader)
			cs := newClientStream(ctx, ctrans, streamFrame{sid: genStreamID(), method: method})
			cs.rpcInfo = rpcinfo.NewRPCInfo(
				rpcinfo.NewEndpointInfo(cliSvcName, method, nil, nil), nil, nil, nil, nil)
			cs.setStreamTimeout(tm)
			err = ctrans.WriteStream(ctx, cs, intHeader, strHeader)
			test.Assert(t, err == nil, err)
			strans := newServerTransport(sconn)
			defer func() {
				strans.Close(nil)
				strans.WaitClosed()
			}()
			ss, err := strans.ReadStream(context.Background())
			test.Assert(t, err == nil, err)

			var wg sync.WaitGroup
			wg.Add(1)
			// client
			go func() {
				defer wg.Done()
				for i := 0; i < 10; i++ {
					req := new(testRequest)
					req.A = int32(i)
					req.B = "hello"
					cErr := cs.SendMsg(ctx, req)
					test.Assert(t, cErr == nil, cErr)
				}

				cErr := cs.CloseSend(ctx)
				test.Assert(t, cErr == nil, cErr)

				// Wait for final response, should timeout
				res := new(testResponse)
				cErr = cs.RecvMsg(ctx, res)
				test.Assert(t, cErr != nil)
				t.Logf("client-side Stream Recv err: %v", cErr)
				test.Assert(t, errors.Is(cErr, kerrors.ErrStreamingTimeout), cErr)
			}()

			// server
			checkServerSideStreamTimeout(t, ss, start, tm)

			// Receive all client messages
			for {
				req := new(testRequest)
				sErr := ss.RecvMsg(ss.ctx, req)
				if sErr == io.EOF {
					break
				}
				test.Assert(t, sErr == nil, sErr)
			}

			// Server delays a lot before sending final response
			time.Sleep(100 * time.Millisecond)

			res := new(testResponse)
			res.B = "done"
			sErr := ss.SendMsg(ss.ctx, res)
			if sErr != nil {
				// Send will fail because client already timeout
				t.Logf("server-side Stream Send err: %v", sErr)
				test.Assert(t, errors.Is(sErr, kerrors.ErrStreamingTimeout), sErr)
			}
			wg.Wait()
		})
	})
	t.Run("BidiStreaming", func(t *testing.T) {
		method := "BidiStreaming"
		t.Run("configure timeout and finish in time", func(t *testing.T) {
			cfd, sfd := netpoll.GetSysFdPairs()
			cconn, err := netpoll.NewFDConnection(cfd)
			test.Assert(t, err == nil, err)
			sconn, err := netpoll.NewFDConnection(sfd)
			test.Assert(t, err == nil, err)

			intHeader := make(IntHeader)
			strHeader := make(streaming.Header)
			ctrans := newClientTransport(cconn, nil)
			defer func() {
				ctrans.Close(nil)
				ctrans.WaitClosed()
			}()
			start := time.Now()
			tm := 200 * time.Millisecond
			ctx, cancel := context.WithTimeout(context.Background(), tm)
			defer cancel()
			injectStreamTimeout(ctx, intHeader)
			cs := newClientStream(ctx, ctrans, streamFrame{sid: genStreamID(), method: method})
			cs.rpcInfo = rpcinfo.NewRPCInfo(
				rpcinfo.NewEndpointInfo(cliSvcName, method, nil, nil), nil, nil, nil, nil)
			cs.setStreamTimeout(tm)
			err = ctrans.WriteStream(ctx, cs, intHeader, strHeader)
			test.Assert(t, err == nil, err)
			strans := newServerTransport(sconn)
			defer func() {
				strans.Close(nil)
				strans.WaitClosed()
			}()
			ss, err := strans.ReadStream(context.Background())
			test.Assert(t, err == nil, err)

			var wg sync.WaitGroup
			wg.Add(1)
			// client
			go func() {
				defer wg.Done()
				// Send and receive 10 messages
				for i := 0; i < 10; i++ {
					req := new(testRequest)
					req.A = int32(i)
					req.B = "hello"
					cErr := cs.SendMsg(ctx, req)
					test.Assert(t, cErr == nil, cErr)

					res := new(testResponse)
					cErr = cs.RecvMsg(ctx, res)
					test.Assert(t, cErr == nil, cErr)
					test.DeepEqual(t, req.A, res.A)
				}

				cErr := cs.CloseSend(ctx)
				test.Assert(t, cErr == nil, cErr)
			}()

			// server
			checkServerSideStreamTimeout(t, ss, start, tm)

			for i := 0; i < 10; i++ {
				req := new(testRequest)
				sErr := ss.RecvMsg(ss.ctx, req)
				test.Assert(t, sErr == nil, sErr)

				res := new(testResponse)
				res.A = req.A
				res.B = req.B
				sErr = ss.SendMsg(ss.ctx, res)
				test.Assert(t, sErr == nil, sErr)
			}

			err = ss.CloseSend(nil)
			test.Assert(t, err == nil, err)
			wg.Wait()
		})
		t.Run("configure timeout and server-side continue sending", func(t *testing.T) {
			cfd, sfd := netpoll.GetSysFdPairs()
			cconn, err := netpoll.NewFDConnection(cfd)
			test.Assert(t, err == nil, err)
			sconn, err := netpoll.NewFDConnection(sfd)
			test.Assert(t, err == nil, err)

			intHeader := make(IntHeader)
			strHeader := make(streaming.Header)
			ctrans := newClientTransport(cconn, nil)
			defer func() {
				ctrans.Close(nil)
				ctrans.WaitClosed()
			}()
			start := time.Now()
			tm := 50 * time.Millisecond
			ctx, cancel := context.WithTimeout(context.Background(), tm)
			defer cancel()
			injectStreamTimeout(ctx, intHeader)
			cs := newClientStream(ctx, ctrans, streamFrame{sid: genStreamID(), method: method})
			cs.rpcInfo = rpcinfo.NewRPCInfo(
				rpcinfo.NewEndpointInfo(cliSvcName, method, nil, nil), nil, nil, nil, nil)
			cs.setStreamTimeout(tm)
			err = ctrans.WriteStream(ctx, cs, intHeader, strHeader)
			test.Assert(t, err == nil, err)
			strans := newServerTransport(sconn)
			defer func() {
				strans.Close(nil)
				strans.WaitClosed()
			}()
			ss, err := strans.ReadStream(context.Background())
			test.Assert(t, err == nil, err)

			var wg sync.WaitGroup
			wg.Add(1)
			// client
			go func() {
				defer wg.Done()
				// Send one request
				req := new(testRequest)
				req.A = 1
				req.B = "hello"
				cErr := cs.SendMsg(ctx, req)
				test.Assert(t, cErr == nil, cErr)

				// Try to receive, should timeout
				for {
					res := new(testResponse)
					cErr = cs.RecvMsg(ctx, res)
					if cErr != nil {
						t.Logf("client-side Stream Recv err: %v", cErr)
						test.Assert(t, errors.Is(cErr, kerrors.ErrStreamingTimeout), cErr)
						break
					}
				}
			}()

			// server
			checkServerSideStreamTimeout(t, ss, start, tm)

			req := new(testRequest)
			sErr := ss.RecvMsg(ss.ctx, req)
			test.Assert(t, sErr == nil, sErr)

			// Server keeps sending slowly
			for i := 0; i < 10; i++ {
				res := new(testResponse)
				res.A = int32(i)
				res.B = req.B
				sErr = ss.SendMsg(ss.ctx, res)
				if sErr != nil {
					// Send will fail after client timeout
					t.Logf("server-side Stream Send err: %v", sErr)
					test.Assert(t, errors.Is(sErr, kerrors.ErrStreamingTimeout), sErr)
					break
				}
				time.Sleep(30 * time.Millisecond)
			}
			wg.Wait()
		})
		t.Run("configure timeout and client-side continue sending", func(t *testing.T) {
			cfd, sfd := netpoll.GetSysFdPairs()
			cconn, err := netpoll.NewFDConnection(cfd)
			test.Assert(t, err == nil, err)
			sconn, err := netpoll.NewFDConnection(sfd)
			test.Assert(t, err == nil, err)

			intHeader := make(IntHeader)
			strHeader := make(streaming.Header)
			ctrans := newClientTransport(cconn, nil)
			defer func() {
				ctrans.Close(nil)
				ctrans.WaitClosed()
			}()
			start := time.Now()
			tm := 50 * time.Millisecond
			ctx, cancel := context.WithTimeout(context.Background(), tm)
			defer cancel()
			injectStreamTimeout(ctx, intHeader)
			cs := newClientStream(ctx, ctrans, streamFrame{sid: genStreamID(), method: method})
			cs.rpcInfo = rpcinfo.NewRPCInfo(
				rpcinfo.NewEndpointInfo(cliSvcName, method, nil, nil), nil, nil, nil, nil)
			cs.setStreamTimeout(tm)
			err = ctrans.WriteStream(ctx, cs, intHeader, strHeader)
			test.Assert(t, err == nil, err)
			strans := newServerTransport(sconn)
			defer func() {
				strans.Close(nil)
				strans.WaitClosed()
			}()
			ss, err := strans.ReadStream(context.Background())
			test.Assert(t, err == nil, err)

			var wg sync.WaitGroup
			wg.Add(1)
			// client
			go func() {
				defer wg.Done()
				// Client keeps sending slowly
				for i := 0; i < 10; i++ {
					req := new(testRequest)
					req.A = int32(i)
					req.B = "hello"
					cErr := cs.SendMsg(ctx, req)
					if cErr != nil {
						// Send will fail after timeout
						t.Logf("client-side Stream Send err: %v", cErr)
						test.Assert(t, errors.Is(cErr, kerrors.ErrStreamingTimeout), cErr)
						break
					}
					time.Sleep(30 * time.Millisecond)
				}
			}()

			// server
			checkServerSideStreamTimeout(t, ss, start, tm)

			// Try to receive, should timeout (cascaded from client)
			for {
				req := new(testRequest)
				sErr := ss.RecvMsg(ss.ctx, req)
				if sErr != nil {
					t.Logf("server-side Stream Recv err: %v", sErr)
					test.Assert(t, errors.Is(sErr, kerrors.ErrStreamingTimeout), sErr)
					break
				}
				// Echo back if no timeout yet
				res := new(testResponse)
				res.A = req.A
				res.B = req.B
				ss.SendMsg(ss.ctx, res)
			}
			wg.Wait()
		})
	})
}

func checkServerSideStreamTimeout(t *testing.T, ss *serverStream, start time.Time, tm time.Duration) {
	test.Assert(t, ss.ctx.Done() != nil, ss.ctx)
	ddl, ok := ss.ctx.Deadline()
	test.Assert(t, ok, ss.ctx)
	test.Assert(t, ddl.Sub(start) <= tm, start, ddl)
}
