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
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bytedance/gopkg/cloud/metainfo"
	"github.com/cloudwego/gopkg/bufiox"
	"github.com/cloudwego/gopkg/protocol/ttheader"
	"github.com/cloudwego/netpoll"

	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
	"github.com/cloudwego/kitex/pkg/streamx"
	terrors "github.com/cloudwego/kitex/pkg/streamx/provider/ttstream/errors"
)

var testTypeKey = "testType"

const (
	testTypeIllegalFrame           = "illegalFrame"
	testTypeUnexpectedHeaderFrame  = "unexpectedHeaderFrame"
	testTypeUnexpectedTrailerFrame = "unexpectedTrailerFrame"
	testTypeIllegalBizErr          = "illegalBizErr"
)

var streamingServiceInfo = &serviceinfo.ServiceInfo{
	ServiceName: "kitex.service.streaming",
	Methods: map[string]serviceinfo.MethodInfo{
		"TriggerStreamErr": serviceinfo.NewMethodInfo(
			func(ctx context.Context, handler, reqArgs, resArgs interface{}) error {
				return nil
				// return streamxserver.InvokeStream[Request, Response](
				//	ctx, serviceinfo.StreamingBidirectional, handler.(streamx.StreamHandler), reqArgs.(streamx.StreamReqArgs), resArgs.(streamx.StreamResArgs))
			},
			nil,
			nil,
			false,
			serviceinfo.WithStreamingMode(serviceinfo.StreamingBidirectional),
		),
	},
	Extra: map[string]interface{}{"streamingFlag": true, "streamx": true},
}

type illegalFrameType int32

const (
	MissStreamingFlag illegalFrameType = iota
)

func encodeIllegalFrame(t *testing.T, ctx context.Context, writer bufiox.Writer, fr *Frame, flag illegalFrameType) {
	var param ttheader.EncodeParam
	written := writer.WrittenLen()
	switch flag {
	case MissStreamingFlag:
		param = ttheader.EncodeParam{
			SeqID:      fr.sid,
			ProtocolID: ttheader.ProtocolIDThriftStruct,
		}
		param.IntInfo = fr.meta
		if param.IntInfo == nil {
			param.IntInfo = make(IntHeader)
		}
		param.IntInfo[ttheader.FrameType] = frameTypeToString[fr.typ]
		param.IntInfo[ttheader.ToMethod] = fr.method
		totalLenField, err := ttheader.Encode(ctx, param, writer)
		if err != nil {
			t.Errorf("ttheader Encode failed, err: %v", err)
		}
		written = writer.WrittenLen() - written
		binary.BigEndian.PutUint32(totalLenField, uint32(written-4))
	}
}

func TestStreamCloseScenario(t *testing.T) {
	klog.SetLevel(klog.LevelDebug)
	addr := test.GetLocalAddress()
	nAddr, err := net.ResolveTCPAddr("tcp", addr)
	test.Assert(t, err == nil, err)
	ln, err := netpoll.CreateListener("tcp", addr)
	test.Assert(t, err == nil, err)
	defer ln.Close()
	sp, err := NewServerProvider(streamingServiceInfo)
	test.Assert(t, err == nil, err)
	var count int32
	onConnect := func(ctx context.Context, conn netpoll.Connection) context.Context {
		t.Logf("connectionc count number: %d", atomic.AddInt32(&count, 1))
		ctx, err := sp.OnActive(ctx, conn)
		test.Assert(t, err == nil, err)
		nctx, ss, nerr := sp.OnStream(ctx, conn)
		test.Assert(t, nerr == nil, nerr)
		go func() {
			rawss := ss.(*serverStream)
			testType, ok := metainfo.GetValue(nctx, testTypeKey)
			test.Assert(t, ok)
			switch testType {
			case testTypeIllegalFrame:
				encodeIllegalFrame(t, nctx, newWriterBuffer(rawss.trans.conn.Writer()), &Frame{
					streamFrame: streamFrame{
						sid: rawss.sid,
					},
					typ: headerFrameType,
				}, MissStreamingFlag)
				rawss.trans.conn.Writer().Flush()
			case testTypeUnexpectedHeaderFrame:
				hd := streamx.Header{
					"key": "val",
				}
				rawss.trans.streamSendHeader(rawss.stream, rawss.stream.method, hd)
				rawss.trans.streamSendHeader(rawss.stream, rawss.stream.method, hd)
			case testTypeUnexpectedTrailerFrame:
				rawss.trans.streamCloseSend(rawss.stream, rawss.stream.method, nil, nil)
				rawss.trans.streamCloseSend(rawss.stream, rawss.stream.method, nil, nil)
			case testTypeIllegalBizErr:
				err = rawss.appendTrailer(
					"biz-status", "1",
					"biz-message", "message",
					"biz-extra", "invalid extra JSON str",
				)
				test.Assert(t, err == nil, err)
				err = rawss.sendTrailer(nctx, nil)
				test.Assert(t, err == nil, err)
			}
		}()
		return nctx
	}
	loop, err := netpoll.NewEventLoop(nil,
		netpoll.WithOnConnect(onConnect),
		netpoll.WithReadTimeout(10*time.Second),
	)
	test.Assert(t, err == nil, err)
	go func() {
		if err := loop.Serve(ln); err != nil {
			t.Logf("server failed, err: %v", err)
		}
	}()
	test.WaitServerStart(addr)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		if err := loop.Shutdown(ctx); err != nil {
			t.Logf("netpoll eventloop shutdown failed, err: %v", err)
		}
	}()

	cp, err := NewClientProvider(streamingServiceInfo)
	test.Assert(t, err == nil, err)
	cctx := context.Background()
	cfg := rpcinfo.NewRPCConfig()
	cfg.(rpcinfo.MutableRPCConfig).SetStreamRecvTimeout(10 * time.Second)
	svcName := "a.b.c"
	ri := rpcinfo.NewRPCInfo(
		rpcinfo.NewEndpointInfo(svcName, "TriggerStreamErr", nAddr, nil),
		rpcinfo.NewEndpointInfo(svcName, "TriggerStreamErr", nAddr, nil),
		rpcinfo.NewInvocation("svcName", "TriggerStreamErr"),
		cfg,
		rpcinfo.NewRPCStats(),
	)

	t.Run("Illegal Frame", func(t *testing.T) {
		t.Run("Non-streaming Frame", func(t *testing.T) {
			// 每次的 ctx 都应该用新的
			nctx := metainfo.WithValue(cctx, testTypeKey, testTypeIllegalFrame)
			cs, err := cp.NewStream(nctx, ri)
			test.Assert(t, err == nil, err)
			rawcs := cs.(*clientStream)
			err = rawcs.RecvMsg(nctx, nil)
			test.Assert(t, errors.Is(err, terrors.ErrIllegalFrame), err)
			err = rawcs.SendMsg(nctx, nil)
			test.Assert(t, errors.Is(err, terrors.ErrIllegalFrame), err)
		})
	})

	t.Run("Illegal Header Frame", func(t *testing.T) {
		t.Run("Receive multiple header", func(t *testing.T) {
			nctx := metainfo.WithValue(cctx, testTypeKey, testTypeUnexpectedHeaderFrame)
			cs, err := cp.NewStream(nctx, ri)
			test.Assert(t, err == nil, err)
			rawcs := cs.(*clientStream)
			err = rawcs.RecvMsg(nctx, nil)
			test.Assert(t, errors.Is(err, terrors.ErrUnexpectedHeader), err)
			err = rawcs.RecvMsg(nctx, nil)
			test.Assert(t, errors.Is(err, terrors.ErrUnexpectedHeader), err)
			err = rawcs.SendMsg(nctx, nil)
			test.Assert(t, errors.Is(err, terrors.ErrUnexpectedHeader), err)
			err = rawcs.SendMsg(nctx, nil)
			test.Assert(t, errors.Is(err, terrors.ErrUnexpectedHeader), err)
		})
		// todo: 考虑当 Recv 收到 EOF 时，Send 是否也要终结
		t.Run("Receive multiple trailer", func(t *testing.T) {
			nctx := metainfo.WithValue(cctx, testTypeKey, testTypeUnexpectedTrailerFrame)
			cs, err := cp.NewStream(nctx, ri)
			test.Assert(t, err == nil, err)
			rawcs := cs.(*clientStream)
			err = rawcs.RecvMsg(nctx, nil)
			test.Assert(t, errors.Is(err, io.EOF), err)
			err = rawcs.RecvMsg(nctx, nil)
			test.Assert(t, errors.Is(err, io.EOF), err)
			// 这里 Send 不能为 nil
			time.Sleep(100 * time.Millisecond)
			err = rawcs.SendMsg(nctx, nil)
			test.Assert(t, errors.Is(err, io.EOF), err)
			err = rawcs.SendMsg(nctx, nil)
			test.Assert(t, errors.Is(err, io.EOF), err)
		})
	})

	t.Run("Illegal Trailer Frame", func(t *testing.T) {
		t.Run("Illegal BizErr", func(t *testing.T) {
			nctx := metainfo.WithValue(cctx, testTypeKey, testTypeIllegalBizErr)
			cs, err := cp.NewStream(nctx, ri)
			test.Assert(t, err == nil, err)
			rawcs := cs.(*clientStream)
			err = rawcs.RecvMsg(nctx, nil)
			test.Assert(t, errors.Is(err, terrors.ErrIllegalBizErr), err)
			err = rawcs.RecvMsg(nctx, nil)
			test.Assert(t, errors.Is(err, terrors.ErrIllegalBizErr), err)
		})
	})
}
