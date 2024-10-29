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
	"testing"
	"time"

	"github.com/bytedance/gopkg/cloud/metainfo"
	"github.com/cloudwego/gopkg/bufiox"
	"github.com/cloudwego/gopkg/protocol/thrift"
	"github.com/cloudwego/gopkg/protocol/ttheader"
	"github.com/cloudwego/netpoll"

	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
	"github.com/cloudwego/kitex/pkg/streamx"
	terrors "github.com/cloudwego/kitex/pkg/streamx/provider/ttstream/errors"
)

const (
	testTypeKey = "testType"

	testTypeIllegalFrame           = "illegalFrame"
	testTypeUnexpectedHeaderFrame  = "unexpectedHeaderFrame"
	testTypeUnexpectedTrailerFrame = "unexpectedTrailerFrame"
	testTypeIllegalBizErr          = "illegalBizErr"
	testTypeApplicationException   = "applicationException"
)

var streamingServiceInfo = &serviceinfo.ServiceInfo{
	ServiceName: "kitex.service.streaming",
	Methods: map[string]serviceinfo.MethodInfo{
		"TriggerStreamErr": serviceinfo.NewMethodInfo(
			func(ctx context.Context, handler, reqArgs, resArgs interface{}) error {
				return nil
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

func TestErrorScenario(t *testing.T) {
	klog.SetLevel(klog.LevelDebug)
	addr := test.GetLocalAddress()
	nAddr, err := net.ResolveTCPAddr("tcp", addr)
	test.Assert(t, err == nil, err)
	ln, err := netpoll.CreateListener("tcp", addr)
	test.Assert(t, err == nil, err)
	defer ln.Close()
	sp, err := NewServerProvider(streamingServiceInfo)
	test.Assert(t, err == nil, err)
	onConnect := func(ctx context.Context, conn netpoll.Connection) context.Context {
		nctx, err := sp.OnActive(ctx, conn)
		test.Assert(t, err == nil, err)
		nctx, ss, nerr := sp.OnStream(nctx, conn)
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
				rawss.trans.streamSendHeader(rawss.stream, hd)
				rawss.trans.streamSendHeader(rawss.stream, hd)
			case testTypeUnexpectedTrailerFrame:
				rawss.trans.streamCloseSend(rawss.stream, nil, nil)
				rawss.trans.streamCloseSend(rawss.stream, nil, nil)
			case testTypeIllegalBizErr:
				err = rawss.writeTrailer(
					streamx.Trailer{
						"biz-status":  "1",
						"biz-message": "message",
						"biz-extra":   "invalid extra JSON str",
					},
				)
				test.Assert(t, err == nil, err)
				err = rawss.sendTrailer(nctx, nil)
				test.Assert(t, err == nil, err)
			case testTypeApplicationException:
				exception := thrift.NewApplicationException(remote.InternalError, "testApplicationException")
				err = rawss.sendTrailer(nctx, exception)
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
	method := "TriggerStreamErr"
	ri := rpcinfo.NewRPCInfo(
		rpcinfo.NewEndpointInfo(streamingServiceInfo.ServiceName, method, nAddr, nil),
		rpcinfo.NewEndpointInfo(streamingServiceInfo.ServiceName, method, nAddr, nil),
		rpcinfo.NewInvocation(streamingServiceInfo.ServiceName, method),
		cfg,
		rpcinfo.NewRPCStats(),
	)

	t.Run("Illegal Frame", func(t *testing.T) {
		t.Run("Non-streaming Frame", func(t *testing.T) {
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
			err = rawcs.SendMsg(nctx, nil)
			test.Assert(t, errors.Is(err, terrors.ErrUnexpectedHeader), err)
		})
		t.Run("Receive multiple trailer", func(t *testing.T) {
			nctx := metainfo.WithValue(cctx, testTypeKey, testTypeUnexpectedTrailerFrame)
			cs, err := cp.NewStream(nctx, ri)
			test.Assert(t, err == nil, err)
			rawcs := cs.(*clientStream)
			err = rawcs.RecvMsg(nctx, nil)
			test.Assert(t, errors.Is(err, io.EOF), err)
			// wait for second trailer frame
			time.Sleep(50 * time.Millisecond)
			err = rawcs.SendMsg(nctx, nil)
			test.Assert(t, errors.Is(err, io.EOF), err)
		})
	})

	t.Run("Trailer Frame", func(t *testing.T) {
		t.Run("Illegal BizErr", func(t *testing.T) {
			nctx := metainfo.WithValue(cctx, testTypeKey, testTypeIllegalBizErr)
			cs, err := cp.NewStream(nctx, ri)
			test.Assert(t, err == nil, err)
			rawcs := cs.(*clientStream)
			err = rawcs.RecvMsg(nctx, nil)
			test.Assert(t, errors.Is(err, terrors.ErrIllegalBizErr), err)
			err = rawcs.SendMsg(nctx, nil)
			test.Assert(t, errors.Is(err, terrors.ErrIllegalBizErr), err)
		})
		t.Run("Application Exception", func(t *testing.T) {
			nctx := metainfo.WithValue(cctx, testTypeKey, testTypeApplicationException)
			cs, err := cp.NewStream(nctx, ri)
			test.Assert(t, err == nil, err)
			rawcs := cs.(*clientStream)
			err = rawcs.RecvMsg(nctx, nil)
			test.Assert(t, errors.Is(err, terrors.ErrApplicationException), err)
		})
	})
}
