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
	"fmt"
	"sync/atomic"
	"time"

	"github.com/bytedance/gopkg/lang/mcache"
	"github.com/cloudwego/gopkg/protocol/thrift"
	"github.com/cloudwego/gopkg/protocol/ttheader"

	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/streaming"
	"github.com/cloudwego/kitex/pkg/transmeta"
	ktransport "github.com/cloudwego/kitex/transport"
)

var (
	_ streaming.ClientStream          = (*clientStream)(nil)
	_ streaming.ServerStream          = (*serverStream)(nil)
	_ streaming.CloseCallbackRegister = (*stream)(nil)
)

func newClientSideStream(ctx context.Context, writer streamWriter, smeta streamFrame) *stream {
	s := newBasicStream(ctx, writer, smeta)
	s.side = clientSide
	s.reader = newStreamReaderWithCtxDoneCallback(s.ctxDoneCallback)
	return s
}

func newServerSideStream(ctx context.Context, writer streamWriter, smeta streamFrame) *stream {
	s := newBasicStream(ctx, writer, smeta)
	s.side = serverSide
	s.reader = newStreamReader()
	return s
}

// newBasicStream is a common function creating stream basic fields,
// pls use newClientSideStream or newServerSideStream to create the real stream
func newBasicStream(ctx context.Context, writer streamWriter, smeta streamFrame) *stream {
	s := new(stream)
	s.ctx = ctx
	s.rpcInfo = rpcinfo.GetRPCInfo(ctx)
	s.streamFrame = smeta
	s.writer = writer
	s.wheader = make(streaming.Header)
	s.wtrailer = make(streaming.Trailer)
	s.headerSig = make(chan int32, 1)
	s.trailerSig = make(chan int32, 1)
	return s
}

// streamFrame define a basic stream frame
type streamFrame struct {
	sid     int32
	method  string
	meta    IntHeader
	header  streaming.Header // key:value, key is full name
	trailer streaming.Trailer
}

const (
	streamSigNone     int32 = 0
	streamSigActive   int32 = 1
	streamSigInactive int32 = -1
	streamSigCancel   int32 = -2
)

// streamState transition
// todo: using AI to draw streamState transition
const (
	streamStateActive          int32 = 0 // when stream is created, init state is active
	streamStateHalfCloseLocal  int32 = 1
	streamStateHalfCloseRemote int32 = 2
	streamStateInactive        int32 = 3
)

// stream is used to process frames and expose user APIs
type stream struct {
	streamFrame
	side     sideType
	ctx      context.Context
	rpcInfo  rpcinfo.RPCInfo
	reader   *streamReader
	writer   streamWriter
	wheader  streaming.Header  // wheader == nil means it already be sent
	wtrailer streaming.Trailer // wtrailer == nil means it already be sent

	headerSig  chan int32
	trailerSig chan int32

	state                 int32
	clientStreamException atomic.Value     // type must be of *exceptionType, only valid in client-side stream
	cancelFunc            cancelWithReason // only valid in server-side stream

	recvTimeout      time.Duration
	metaFrameHandler MetaFrameHandler
	closeCallback    []func(error)
}

func (s *stream) Service() string {
	if len(s.header) == 0 {
		return ""
	}
	return s.header[ttheader.HeaderIDLServiceName]
}

func (s *stream) Method() string {
	return s.method
}

func (s *stream) TransportProtocol() ktransport.Protocol {
	return ktransport.TTHeaderStreaming
}

// SendMsg send a message to peer.
// In order to avoid underlying execution errors when the context passed in by the user does not
// contain information related to this RPC, the context specified when creating the stream is used
// here, and the context passed in by the user is ignored.
func (s *stream) SendMsg(ctx context.Context, msg any) (err error) {
	// encode payload
	payload, err := EncodePayload(s.ctx, msg)
	if err != nil {
		return err
	}
	// tracing send size
	ri := s.rpcInfo
	if ri != nil && ri.Stats() != nil {
		if rpcStats := rpcinfo.AsMutableRPCStats(ri.Stats()); rpcStats != nil {
			rpcStats.IncrSendSize(uint64(len(payload)))
		}
	}
	// send data frame
	return s.writeFrame(dataFrameType, nil, nil, payload)
}

func (s *stream) RecvMsg(ctx context.Context, data any) error {
	if s.recvTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, s.recvTimeout)
		defer cancel()
	}
	payload, err := s.reader.output(ctx)
	if err != nil {
		return err
	}
	err = DecodePayload(ctx, payload, data)
	// payload will not be access after decode
	mcache.Free(payload)

	// tracing recv size
	ri := s.rpcInfo
	if ri != nil && ri.Stats() != nil {
		if rpcStats := rpcinfo.AsMutableRPCStats(ri.Stats()); rpcStats != nil {
			rpcStats.IncrRecvSize(uint64(len(payload)))
		}
	}
	return err
}

// ctxDoneCallback convert ctx.Err() to ttstream related err and close client-side stream.
// it is invoked in container.Pipe
func (s *stream) ctxDoneCallback(ctx context.Context) {
	var finalEx tException
	var initialNode string
	cErr := ctx.Err()
	switch cErr {
	// biz code invokes cancel()
	case context.Canceled:
		finalEx = errBizCancel.NewBuilder().WithSide(clientSide)
		if s.rpcInfo != nil {
			initialNode = s.rpcInfo.From().ServiceName()
		}
	default:
		if tEx, ok := cErr.(tException); ok {
			// 此处需要 copy 出来一个新的错误出来，否则显示的是 server-side stream，这是错误的，必须是 client-side stream
			finalEx = tEx
			initialNode = tEx.TriggeredBy()
		} else {
			finalEx = errInternalCancel.NewBuilder().WithSide(clientSide).WithCause(cErr)
		}
	}

	s.close(finalEx, true, initialNode)
}

func (s *stream) RegisterCloseCallback(cb func(error)) {
	s.closeCallback = append(s.closeCallback, cb)
}

// closeSend should be called when following cases happen:
// client:
// - user call CloseSend
// - client recv a trailer
// - transport layer exception
// server:
// - server handler return
// - transport layer exception
func (s *stream) closeSend(exception error) error {
	err := s.sendTrailer(exception)
	return err
}

// closeRecv called only when server receiving Trailer Frame
func (s *stream) closeRecv(exception error) error {
	// client->server data transferring has stopped
	atomic.CompareAndSwapInt32(&s.state, streamStateActive, streamStateHalfCloseRemote)
	select {
	case s.headerSig <- streamSigInactive:
	default:
	}
	select {
	case s.trailerSig <- streamSigInactive:
	default:
	}
	s.reader.close(exception)
	return nil
}

func (s *stream) close(exception error, sendRst bool, initialNode string) error {
	oldState := atomic.SwapInt32(&s.state, streamStateInactive)
	if oldState == streamStateInactive {
		// stream has been closed before
		return nil
	}
	select {
	case s.headerSig <- streamSigInactive:
	default:
	}
	select {
	case s.trailerSig <- streamSigInactive:
	default:
	}
	s.reader.close(exception)
	if sendRst {
		s.sendRst(exception, initialNode)
	}
	if s.side == clientSide {
		if exception != nil {
			s.clientStreamException.Store(exception)
		}
		if oldState != streamStateHalfCloseLocal {
			// For client-side stream, if trailer frame is received, finish the lifecycle directly.
			// But Some downstream components may not be upgraded and need to receive a Trailer Frame to end the Stream's lifecycle.
			// For better compatibility, we choose to send a Trailer Frame when the client-side Stream is closed to.
			//
			// remove this logic in the future.
			s.closeSend(nil)
		}
	} else {
		// for server-side stream, support cascading cancel
		s.cancelFunc(exception)
	}
	s.runCloseCallback(exception)
	return nil
}

func (s *stream) setRecvTimeout(timeout time.Duration) {
	if timeout <= 0 {
		return
	}
	s.recvTimeout = timeout
}

func (s *stream) setMetaFrameHandler(metaHandler MetaFrameHandler) {
	s.metaFrameHandler = metaHandler
}

func (s *stream) runCloseCallback(exception error) {
	if len(s.closeCallback) > 0 {
		for _, cb := range s.closeCallback {
			cb(exception)
		}
	}
	_ = s.writer.CloseStream(s.sid)
}

func (s *stream) writeFrame(ftype int32, header streaming.Header, trailer streaming.Trailer, payload []byte) (err error) {
	fr := newFrame(streamFrame{sid: s.sid, method: s.method, header: header, trailer: trailer}, ftype, payload)
	return s.writer.WriteFrame(fr)
}

// writeHeader copy kvs into s.wheader
func (s *stream) writeHeader(hd streaming.Header) error {
	if s.wheader == nil {
		return fmt.Errorf("stream header already sent")
	}
	for k, v := range hd {
		s.wheader[k] = v
	}
	return nil
}

// sendHeader send header to peer
func (s *stream) sendHeader() (err error) {
	wheader := s.wheader
	s.wheader = nil
	if wheader == nil {
		return fmt.Errorf("stream header already sent")
	}
	err = s.writeFrame(headerFrameType, wheader, nil, nil)
	return err
}

// writeTrailer write trailer to peer
func (s *stream) writeTrailer(tl streaming.Trailer) (err error) {
	if s.wtrailer == nil {
		return fmt.Errorf("stream trailer already sent")
	}
	for k, v := range tl {
		s.wtrailer[k] = v
	}
	return nil
}

// writeTrailer send trailer to peer
// if exception is not nil, trailer frame should carry a payload
func (s *stream) sendTrailer(exception error) (err error) {
	wtrailer := s.wtrailer
	s.wtrailer = nil
	if wtrailer == nil {
		return fmt.Errorf("stream trailer already sent")
	}
	klog.Debugf("stream[%d] send trailer: err=%v", s.sid, exception)

	var payload []byte
	if exception != nil {
		payload, err = EncodeException(context.Background(), s.method, s.sid, exception)
		if err != nil {
			return err
		}
	}
	err = s.writeFrame(trailerFrameType, nil, wtrailer, payload)
	return err
}

func (s *stream) sendRst(exception error, initialNode string) (err error) {
	klog.Debugf("stream[%d] send rst: err=%v", s.sid, exception)
	var payload []byte
	if exception != nil {
		payload, err = EncodeException(context.Background(), s.method, s.sid, exception)
		if err != nil {
			// todo: log but not return err
			return err
		}
	}
	var header streaming.Header
	if initialNode != "" {
		header = make(streaming.Header)
		header[ttheader.HeaderCascadingLinkInitialNode] = initialNode
	}
	return s.writeFrame(rstFrameType, header, nil, payload)
}

// only for test
func (s *stream) sendRstFrame(exception error, initialNode string) error {
	var payload []byte
	var err error
	if exception != nil {
		payload, err = EncodeException(context.Background(), s.method, s.sid, exception)
		if err != nil {
			return err
		}
	}
	var header streaming.Header
	if initialNode != "" {
		header = make(streaming.Header)
		header[ttheader.HeaderCascadingLinkInitialNode] = initialNode
	}
	return s.writeFrame(rstFrameType, header, nil, payload)
}

// === Frame OnRead callback

func (s *stream) onReadMetaFrame(fr *Frame) (err error) {
	if s.metaFrameHandler == nil {
		return nil
	}
	return s.metaFrameHandler.OnMetaFrame(s.ctx, fr.meta, fr.header, fr.payload)
}

func (s *stream) onReadHeaderFrame(fr *Frame) (err error) {
	if s.header != nil {
		return errUnexpectedHeader.WithCause(fmt.Errorf("stream[%d] already set header", s.sid))
	}
	s.header = fr.header
	select {
	case s.headerSig <- streamSigActive:
	default:
		return errUnexpectedHeader.WithCause(fmt.Errorf("stream[%d] already set header", s.sid))
	}
	klog.Debugf("stream[%s] read header: %v", s.method, fr.header)
	return nil
}

func (s *stream) onReadDataFrame(fr *Frame) (err error) {
	s.reader.input(context.Background(), fr.payload)
	return nil
}

// onReadTrailerFrame by client: unblock recv function and return EOF if no unread frame
// onReadTrailerFrame by server: unblock recv function and return EOF if no unread frame
func (s *stream) onReadTrailerFrame(fr *Frame) (err error) {
	var exception error
	// when server-side returns non-biz error, it will be wrapped as ApplicationException stored in trailer frame payload
	if len(fr.payload) > 0 {
		// exception is type of (*thrift.ApplicationException)
		_, _, err = thrift.UnmarshalFastMsg(fr.payload, nil)
		exception = errApplicationException.WithCause(err)
	} else if len(fr.trailer) > 0 {
		// when server-side returns biz error, payload is empty and biz error information is stored in trailer frame header
		bizErr, err := transmeta.ParseBizStatusErr(fr.trailer)
		if err != nil {
			exception = errIllegalBizErr.WithCause(err)
		} else if bizErr != nil {
			// bizErr is independent of rpc exception handling
			exception = bizErr
		}
	}
	s.trailer = fr.trailer
	select {
	case s.trailerSig <- streamSigActive:
	default:
	}
	select {
	case s.headerSig <- streamSigNone:
		// if trailer arrived, we should return unblock stream.Header()
	default:
	}

	klog.Debugf("stream[%d] recv trailer: %v, exception: %v", s.sid, s.trailer, exception)
	// client-side stream recv trailer, the lifecycle of whole stream has ended
	if s.side == clientSide {
		return s.close(exception, false, "")
	}
	// server-side stream recv trailer, we only need to close recv but still can send data
	return s.closeRecv(exception)
}

var defaultRstException = thrift.NewApplicationException(13, "rst")

func (s *stream) onReadRstFrame(fr *Frame) (err error) {
	var rstEx *canceledExceptionType
	var ex *thrift.ApplicationException
	if len(fr.payload) > 0 {
		// exception is type of (*thrift.ApplicationException)
		ex, err = unmarshalApplicationException(fr.payload)
		if err != nil {
			return err
		}
	} else {
		klog.Infof("KITEX: stream[%d] recv rst frame without payload", s.sid)
		ex = defaultRstException
	}
	var by string
	if fr.header != nil {
		// cascading link initial node
		clin, ok := fr.header[ttheader.HeaderCascadingLinkInitialNode]
		if ok {
			by = clin
		}
	}
	if s.side == clientSide {
		rstEx = errDownstreamCancel.NewBuilder().WithSide(s.side)
	} else {
		rstEx = errUpstreamCancel.NewBuilder().WithSide(s.side)
	}
	if by != "" {
		rstEx = rstEx.WithTriggeredBy(by).WithCauseAndTypeId(ex, ex.TypeId())
	} else {
		rstEx = rstEx.WithCauseAndTypeId(ex, ex.TypeId())
	}
	// todo: compare with gRPC to deal with header/trailer behaviour
	s.trailer = fr.trailer
	select {
	case s.trailerSig <- streamSigActive:
	default:
	}
	select {
	case s.headerSig <- streamSigNone:
		// if trailer arrived, we should return unblock stream.Header()
	default:
	}

	klog.Debugf("stream[%d] recv trailer: %v, exception: %v", s.sid, s.trailer, rstEx)
	// when receiving rst frame, we should close stream and there is no need to send rst frame
	err = s.close(rstEx, false, "")
	return err
}
