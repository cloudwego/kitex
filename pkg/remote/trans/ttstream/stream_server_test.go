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
	"sync"
	"testing"
	"time"

	"github.com/cloudwego/gopkg/protocol/ttheader"

	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/streaming"
)

func newTestServerStream() *serverStream {
	return newTestServerStreamWithStreamWriter(mockStreamWriter{})
}

func newTestServerStreamWithStreamWriter(w streamWriter) *serverStream {
	ctx, cancel := context.WithCancel(context.Background())
	ctx, cancelFunc := newContextWithCancelReason(ctx, cancel)
	srvSt := newServerStream(ctx, w, streamFrame{}, ttheader.ProtocolIDThriftStruct)
	srvSt.cancelFunc = cancelFunc
	return srvSt
}

func Test_serverStreamStateChange(t *testing.T) {
	t.Run("serverStream close, then RecvMsg/SendMsg returning exception", func(t *testing.T) {
		srvSt := newTestServerStream()
		testException := errInternalCancel.newBuilder().withCause(errors.New("test"))
		err := srvSt.close(testException)
		test.Assert(t, err == nil, err)
		rErr := srvSt.RecvMsg(srvSt.ctx, nil)
		test.Assert(t, rErr == testException, rErr)
		sErr := srvSt.SendMsg(srvSt.ctx, nil)
		test.Assert(t, sErr == testException, sErr)
	})
	t.Run("serverStream close twice with different exception, RecvMsg/SendMsg returning the first time exception", func(t *testing.T) {
		srvSt := newTestServerStream()
		testException1 := errInternalCancel.newBuilder().withCause(errors.New("test1"))
		err := srvSt.close(testException1)
		test.Assert(t, err == nil, err)
		testException2 := errInternalCancel.newBuilder().withCause(errors.New("test2"))
		err = srvSt.close(testException2)
		test.Assert(t, err == nil, err)

		rErr := srvSt.RecvMsg(srvSt.ctx, nil)
		test.Assert(t, rErr == testException1, rErr)
		sErr := srvSt.SendMsg(srvSt.ctx, nil)
		test.Assert(t, sErr == testException1, sErr)
	})
	t.Run("serverStream close concurrently with RecvMsg/SendMsg", func(t *testing.T) {
		srvSt := newTestServerStream()
		var wg sync.WaitGroup
		wg.Add(2)
		testException := errInternalCancel.newBuilder().withCause(errors.New("test1"))
		go func() {
			time.Sleep(100 * time.Millisecond)
			err := srvSt.close(testException)
			test.Assert(t, err == nil, err)
		}()
		go func() {
			defer wg.Done()
			rErr := srvSt.RecvMsg(srvSt.ctx, nil)
			test.Assert(t, rErr == testException, rErr)
		}()
		go func() {
			defer wg.Done()
			for {
				sErr := srvSt.SendMsg(srvSt.ctx, &testRequest{})
				if sErr == nil {
					continue
				}
				test.Assert(t, sErr == testException, sErr)
				break
			}
		}()
		wg.Wait()
	})
}

func Test_serverStreamReimburseHeaderFrame(t *testing.T) {
	t.Run("CloseSend", func(t *testing.T) {
		var frameNum int
		srvSt := newTestServerStreamWithStreamWriter(mockStreamWriter{
			writeFrameFunc: func(f *Frame) error {
				switch f.typ {
				case headerFrameType:
					// first frame
					test.Assert(t, frameNum == 0, f)
					frameNum++
				case trailerFrameType:
					// second frame
					test.Assert(t, frameNum == 1, f)
					frameNum++
				default:
					t.Fatalf("should not send other frame, frame: %+v, frameNum: %d", f, frameNum)
				}
				return nil
			},
		})
		err := srvSt.CloseSend(nil)
		test.Assert(t, err == nil, err)
		test.Assert(t, frameNum == 2, frameNum)
	})
	t.Run("Send -> CloseSend", func(t *testing.T) {
		var frameNum int
		srvSt := newTestServerStreamWithStreamWriter(mockStreamWriter{
			writeFrameFunc: func(f *Frame) error {
				switch f.typ {
				case headerFrameType:
					// first frame
					test.Assert(t, frameNum == 0, f)
					frameNum++
				case dataFrameType:
					// second frame
					test.Assert(t, frameNum == 1, f)
					frameNum++
				case trailerFrameType:
					// second frame
					test.Assert(t, frameNum == 2, f)
					frameNum++
				default:
					t.Fatalf("should not send other frame, frame: %+v, frameNum: %d", f, frameNum)
				}
				return nil
			},
		})
		err := srvSt.SendMsg(context.Background(), new(testResponse))
		test.Assert(t, err == nil, err)
		err = srvSt.CloseSend(nil)
		test.Assert(t, err == nil, err)
		test.Assert(t, frameNum == 3, frameNum)
	})
	t.Run("Send -> Send -> CloseSend", func(t *testing.T) {
		var frameNum int
		srvSt := newTestServerStreamWithStreamWriter(mockStreamWriter{
			writeFrameFunc: func(f *Frame) error {
				switch f.typ {
				case headerFrameType:
					// first frame
					test.Assert(t, frameNum == 0, f)
					frameNum++
				case dataFrameType:
					// second frame
					test.Assert(t, frameNum == 1 || frameNum == 2, f)
					frameNum++
				case trailerFrameType:
					// second frame
					test.Assert(t, frameNum == 3, f)
					frameNum++
				default:
					t.Fatalf("should not send other frame, frame: %+v, frameNum: %d", f, frameNum)
				}
				return nil
			},
		})
		err := srvSt.SendMsg(context.Background(), new(testResponse))
		test.Assert(t, err == nil, err)
		err = srvSt.SendMsg(context.Background(), new(testResponse))
		test.Assert(t, err == nil, err)
		err = srvSt.CloseSend(nil)
		test.Assert(t, err == nil, err)
		test.Assert(t, frameNum == 4, frameNum)
	})
	t.Run("SendHeader -> CloseSend", func(t *testing.T) {
		var frameNum int
		srvSt := newTestServerStreamWithStreamWriter(mockStreamWriter{
			writeFrameFunc: func(f *Frame) error {
				switch f.typ {
				case headerFrameType:
					// first frame
					test.Assert(t, frameNum == 0, f)
					frameNum++
				case trailerFrameType:
					// second frame
					test.Assert(t, frameNum == 1, f)
					frameNum++
				default:
					t.Fatalf("should not send other frame, frame: %+v, frameNum: %d", f, frameNum)
				}
				return nil
			},
		})
		err := srvSt.SendHeader(streaming.Header{"testKey": "testVal"})
		test.Assert(t, err == nil)
		err = srvSt.CloseSend(nil)
		test.Assert(t, err == nil, err)
		test.Assert(t, frameNum == 2, frameNum)
	})
	t.Run("SendHeader -> Send -> CloseSend", func(t *testing.T) {
		var frameNum int
		srvSt := newTestServerStreamWithStreamWriter(mockStreamWriter{
			writeFrameFunc: func(f *Frame) error {
				switch f.typ {
				case headerFrameType:
					// first frame
					test.Assert(t, frameNum == 0, f)
					frameNum++
				case dataFrameType:
					// second frame
					test.Assert(t, frameNum == 1, f)
					frameNum++
				case trailerFrameType:
					// third frame
					test.Assert(t, frameNum == 2, f)
					frameNum++
				default:
					t.Fatalf("should not send other frame, frame: %+v, frameNum: %d", f, frameNum)
				}
				return nil
			},
		})
		err := srvSt.SendHeader(streaming.Header{"testKey": "testVal"})
		test.Assert(t, err == nil)
		err = srvSt.SendMsg(context.Background(), new(testResponse))
		test.Assert(t, err == nil, err)
		err = srvSt.CloseSend(nil)
		test.Assert(t, err == nil, err)
		test.Assert(t, frameNum == 3, frameNum)
	})
}
