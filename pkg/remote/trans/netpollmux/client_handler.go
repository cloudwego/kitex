/*
 * Copyright 2021 CloudWeGo Authors
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

package netpollmux

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync/atomic"

	"github.com/cloudwego/netpoll"

	"github.com/cloudwego/kitex/pkg/streaming"

	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/codec"
	"github.com/cloudwego/kitex/pkg/remote/trans"
	np "github.com/cloudwego/kitex/pkg/remote/trans/netpoll"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
	"github.com/cloudwego/kitex/pkg/stats"
)

type cliTransHandlerFactory struct{}

// NewCliTransHandlerFactory creates a new netpollmux client transport handler factory.
func NewCliTransHandlerFactory() remote.ClientTransHandlerFactory {
	return &cliTransHandlerFactory{}
}

// NewTransHandler implements the remote.ClientTransHandlerFactory interface.
func (f *cliTransHandlerFactory) NewTransHandler(opt *remote.ClientOption) (remote.ClientTransHandler, error) {
	if _, ok := opt.ConnPool.(*MuxPool); !ok {
		return nil, fmt.Errorf("ConnPool[%T] invalid, netpoll mux just support MuxPool", opt.ConnPool)
	}
	return newCliTransHandler(opt)
}

func newCliTransHandler(opt *remote.ClientOption) (*cliTransHandler, error) {
	return &cliTransHandler{
		opt:   opt,
		codec: opt.Codec,
	}, nil
}

var _ remote.ClientTransHandler = &cliTransHandler{}

type cliTransHandler struct {
	opt       *remote.ClientOption
	codec     remote.Codec
	transPipe *remote.ClientTransPipeline
}

func (t *cliTransHandler) NewStream(ctx context.Context, conn net.Conn) (streaming.ClientStream, error) {
	ri := rpcinfo.GetRPCInfo(ctx)
	cs := newCliStream(ctx, conn, t, ri)
	cs.pipe.Initialize(cs, t.transPipe)
	return cs, nil
}

// OnInactive implements the remote.ClientTransHandler interface.
func (t *cliTransHandler) OnInactive(ctx context.Context, conn net.Conn) {
	// ineffective now and do nothing
}

// OnError implements the remote.ClientTransHandler interface.
func (t *cliTransHandler) OnError(ctx context.Context, err error, conn net.Conn) {
	if pe, ok := err.(*kerrors.DetailedError); ok {
		klog.CtxErrorf(ctx, "KITEX: send request error, remote=%s, error=%s\nstack=%s", conn.RemoteAddr(), err.Error(), pe.Stack())
	} else {
		klog.CtxErrorf(ctx, "KITEX: send request error, remote=%s, error=%s", conn.RemoteAddr(), err.Error())
	}
}

// SetPipeline implements the remote.ClientTransHandler interface.
func (t *cliTransHandler) SetPipeline(p *remote.ClientTransPipeline) {
	t.transPipe = p
}

type cliStream struct {
	conn net.Conn
	ctx  context.Context
	t    *cliTransHandler
	ri   rpcinfo.RPCInfo
	pipe remote.ClientStreamPipeline
}

func newCliStream(ctx context.Context, conn net.Conn, t *cliTransHandler, ri rpcinfo.RPCInfo) *cliStream {
	return &cliStream{
		conn: conn,
		ctx:  ctx,
		t:    t,
		ri:   ri,
	}
}

func (s *cliStream) Write(ctx context.Context, sendMsg remote.Message) (nctx context.Context, err error) {
	ri := sendMsg.RPCInfo()
	rpcinfo.Record(ctx, ri, stats.WriteStart, nil)
	buf := netpoll.NewLinkBuffer()
	bufWriter := np.NewWriterByteBuffer(buf)
	defer func() {
		if err != nil {
			buf.Close()
			bufWriter.Release(err)
		}
		rpcinfo.Record(ctx, ri, stats.WriteFinish, nil)
	}()

	// Set header flag = 1
	tags := sendMsg.Tags()
	if tags == nil {
		tags = make(map[string]interface{})
	}
	tags[codec.HeaderFlagsKey] = codec.HeaderFlagSupportOutOfOrder

	// encode
	sendMsg.SetPayloadCodec(s.t.opt.PayloadCodec)
	err = s.t.codec.Encode(ctx, sendMsg, bufWriter)
	if err != nil {
		return ctx, err
	}

	mc, _ := s.conn.(*muxCliConn)

	// if oneway
	var methodInfo serviceinfo.MethodInfo
	if methodInfo, err = trans.GetMethodInfo(ri, ri.Invocation().ServiceInfo()); err != nil {
		return ctx, err
	}
	if methodInfo.OneWay() {
		mc.Put(func() (_ netpoll.Writer, isNil bool) {
			return buf, false
		})
		return ctx, nil
	}

	// add notify
	seqID := ri.Invocation().SeqID()
	callback := newAsyncCallback(buf, bufWriter)
	mc.seqIDMap.store(seqID, callback)
	mc.Put(callback.getter)
	return ctx, err
}

func (s *cliStream) Read(ctx context.Context, msg remote.Message) (nctx context.Context, err error) {
	ri := msg.RPCInfo()
	mc, _ := s.conn.(*muxCliConn)
	seqID := ri.Invocation().SeqID()
	// load & delete before return
	event, _ := mc.seqIDMap.load(seqID)
	defer mc.seqIDMap.delete(seqID)

	callback, _ := event.(*asyncCallback)
	defer callback.Close()

	readTimeout := trans.GetReadTimeout(ri.Config())
	if readTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, readTimeout)
		defer cancel()
	}
	select {
	case <-ctx.Done():
		// timeout
		return ctx, fmt.Errorf("recv wait timeout %s, seqID=%d", readTimeout, seqID)
	case bufReader := <-callback.notifyChan:
		// recv
		if bufReader == nil {
			return ctx, ErrConnClosed
		}
		rpcinfo.Record(ctx, ri, stats.ReadStart, nil)
		msg.SetPayloadCodec(s.t.opt.PayloadCodec)
		err := s.t.codec.Decode(ctx, msg, bufReader)
		if err != nil && errors.Is(err, netpoll.ErrReadTimeout) {
			err = kerrors.ErrRPCTimeout.WithCause(err)
		}
		if l := bufReader.ReadableLen(); l > 0 {
			bufReader.Skip(l)
		}
		bufReader.Release(nil)
		rpcinfo.Record(ctx, ri, stats.ReadFinish, err)
		return ctx, err
	}
}

func (s *cliStream) Header() (streaming.Header, error) {
	panic("not implemented streaming method for default client handler")
}

func (s *cliStream) Trailer() (streaming.Trailer, error) {
	panic("not implemented streaming method for default client handler")
}

func (s *cliStream) CloseSend() error {
	return nil
}

func (s *cliStream) RecvMsg(resp interface{}) (err error) {
	ri := s.ri
	recvMsg := remote.NewMessage(resp, ri.Invocation().ServiceInfo(), ri, remote.Reply, remote.Client)
	defer func() {
		remote.RecycleMessage(recvMsg)
	}()
	_, err = s.pipe.Read(s.ctx, recvMsg)
	return err
}

func (s *cliStream) SendMsg(req interface{}) (err error) {
	ri := s.ri
	ink := ri.Invocation()
	svcInfo := ink.ServiceInfo()
	m := ink.MethodInfo()
	var sendMsg remote.Message
	if m == nil {
		return fmt.Errorf("method info is nil, methodName=%s, serviceInfo=%+v", ink.MethodName(), svcInfo)
	} else if m.OneWay() {
		sendMsg = remote.NewMessage(req, svcInfo, ri, remote.Oneway, remote.Client)
	} else {
		sendMsg = remote.NewMessage(req, svcInfo, ri, remote.Call, remote.Client)
	}
	defer func() {
		remote.RecycleMessage(sendMsg)
	}()
	_, err = s.pipe.Write(s.ctx, sendMsg)
	return err
}

type asyncCallback struct {
	wbuf       *netpoll.LinkBuffer
	bufWriter  remote.ByteBuffer
	notifyChan chan remote.ByteBuffer // notify recv reader
	closed     int32                  // 1 is closed, 2 means wbuf has been flush
}

func newAsyncCallback(wbuf *netpoll.LinkBuffer, bufWriter remote.ByteBuffer) *asyncCallback {
	return &asyncCallback{
		wbuf:       wbuf,
		bufWriter:  bufWriter,
		notifyChan: make(chan remote.ByteBuffer, 1),
	}
}

// Recv is called when receive a message.
func (c *asyncCallback) Recv(bufReader remote.ByteBuffer, err error) error {
	c.notify(bufReader)
	return nil
}

// Close is used to close the mux connection.
func (c *asyncCallback) Close() error {
	if atomic.CompareAndSwapInt32(&c.closed, 0, 1) {
		c.wbuf.Close()
		c.bufWriter.Release(nil)
	}
	return nil
}

func (c *asyncCallback) notify(bufReader remote.ByteBuffer) {
	select {
	case c.notifyChan <- bufReader:
	default:
		if bufReader != nil {
			bufReader.Release(nil)
		}
	}
}

func (c *asyncCallback) getter() (w netpoll.Writer, isNil bool) {
	if atomic.CompareAndSwapInt32(&c.closed, 0, 2) {
		return c.wbuf, false
	}
	return nil, true
}
