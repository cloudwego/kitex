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
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/trans/ttstream/container"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/streaming"
	"github.com/cloudwego/kitex/pkg/transmeta"
	ktransport "github.com/cloudwego/kitex/transport"
)

var (
	_ streaming.ClientStream          = (*clientStream)(nil)
	_ streaming.ServerStream          = (*serverStream)(nil)
	_ streaming.CloseCallbackRegister = (*stream)(nil)

	defaultRstException = thrift.NewApplicationException(remote.InternalError, "stream reset without payload")
)

func newStreamForClientSide(ctx context.Context, writer streamWriter, smeta streamFrame) *stream {
	s := newStream(ctx, writer, smeta)
	s.reader = newStreamReader(
		container.WithCtxDoneCallback(func(ctx context.Context) error {
			return s.convertContextErr(ctx)
		}),
	)
	return s
}

func newStreamForServerSide(ctx context.Context, writer streamWriter, smeta streamFrame) *stream {
	s := newStream(ctx, writer, smeta)
	s.reader = newStreamReader()
	return s
}

func newStream(ctx context.Context, writer streamWriter, smeta streamFrame) *stream {
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

const (
	streamStateActive          int32 = 0
	streamStateHalfCloseLocal  int32 = 1
	streamStateHalfCloseRemote int32 = 2
	streamStateInactive        int32 = 3
)

// stream is used to process frames and expose user APIs
type stream struct {
	streamFrame
	ctx      context.Context
	rpcInfo  rpcinfo.RPCInfo
	reader   *streamReader
	writer   streamWriter
	wheader  streaming.Header  // wheader == nil means it already be sent
	wtrailer streaming.Trailer // wtrailer == nil means it already be sent

	headerSig  chan int32
	trailerSig chan int32

	state                     int32
	transmissionTimeoutEffect bool            // indicates whether transmitted timeout taking effect, only valid in client stream
	clientStreamException     atomic.Value    // type must be of *exceptionType, only valid in client stream
	cancelFunc                cancelWithCause // only valid in server stream

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

func (s *stream) RecvMsg(ctx context.Context, data any, side sideType) error {
	if s.recvTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, s.recvTimeout)
		defer cancel()
	}
	payload, err := s.reader.output(ctx)
	if err != nil {
		// todo: put this close function into reader's callback
		s.close(err, true, side)
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

func (s *stream) RegisterCloseCallback(cb func(error)) {
	s.closeCallback = append(s.closeCallback, cb)
}

// closeSend should be called when following cases happen:
// client:
// - user call CloseSend
// - client recv a trailer for compatibility
// - transport layer exception
func (s *stream) closeSend(exception error) error {
	if !atomic.CompareAndSwapInt32(&s.state, streamStateActive, streamStateHalfCloseLocal) {
		return nil
	}
	return s.sendTrailer(exception)
}

func (s *stream) close(exception error, rst bool, side sideType) error {
	if atomic.SwapInt32(&s.state, streamStateInactive) == streamStateInactive {
		// stream has been closed
		return nil
	}
	if s.cancelFunc != nil {
		s.cancelFunc(exception)
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
	if side == clientTransport && exception != nil {
		s.clientStreamException.Store(exception)
	}
	if rst {
		s.sendRstFrame(exception)
	}
	s.runCloseCallback(exception)
	return nil
}

func (s *stream) cancel() error {
	// todo: simplify this logic
	// since cancel would close this stream directly
	select {
	case s.headerSig <- streamSigCancel:
	default:
	}
	select {
	case s.trailerSig <- streamSigCancel:
	default:
	}
	s.reader.cancel()
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

func (s *stream) setContext(ctx context.Context) {
	s.ctx = ctx
}

func (s *stream) setCancelFunc(f cancelWithCause) {
	s.cancelFunc = f
}

func (s *stream) setTransmissionTimeoutEffect(b bool) {
	s.transmissionTimeoutEffect = b
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
		// todo: replace with exception
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
		// todo: 换成 ttstream exception
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
	return s.writeFrame(trailerFrameType, nil, wtrailer, payload)
}

func (s *stream) sendRstFrame(exception error) (err error) {
	klog.Debugf("stream[%d] send rst frame: err=%v", s.sid, exception)

	var payload []byte
	if exception != nil {
		payload, err = EncodeException(context.Background(), s.method, s.sid, exception)
		if err != nil {
			klog.Errorf("stream[%d] encode exception in rst frame failed: err=%v", s.sid, exception)
			// valid rst frame must be sent otherwise the server stream has the risk of leaking
			payload, _ = EncodeException(context.Background(), s.method, s.sid, defaultRstException)
		}
	}
	return s.writeFrame(rstFrameType, nil, nil, payload)
}

// === Frame OnRead callback

func (s *stream) onReadMetaFrame(fr *Frame) (err error) {
	if s.metaFrameHandler == nil {
		return nil
	}
	return s.metaFrameHandler.OnMetaFrame(s.ctx, fr.meta, fr.header, fr.payload)
}

func (s *stream) onReadHeaderFrame(fr *Frame, side sideType) (err error) {
	if s.header != nil {
		return errUnexpectedHeader.WithSide(side).WithCause(fmt.Errorf("stream[%d] already set header", s.sid))
	}
	s.header = fr.header
	select {
	case s.headerSig <- streamSigActive:
	default:
		return errUnexpectedHeader.WithSide(side).WithCause(fmt.Errorf("stream[%d] already set header", s.sid))
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
func (s *stream) onReadTrailerFrame(fr *Frame, side sideType) (err error) {
	var exception error
	// when server-side returns non-biz error, it will be wrapped as ApplicationException stored in trailer frame payload
	if len(fr.payload) > 0 {
		// exception is type of (*thrift.ApplicationException)
		_, _, err = thrift.UnmarshalFastMsg(fr.payload, nil)
		exception = errApplicationException.WithSide(side).WithCause(err)
	} else if len(fr.trailer) > 0 {
		// when server-side returns biz error, payload is empty and biz error information is stored in trailer frame header
		bizErr, err := transmeta.ParseBizStatusErr(fr.trailer)
		if err != nil {
			exception = errIllegalBizErr.WithSide(side).WithCause(err)
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
	// if client recv trailer, server handler must be return,
	// so we don't need to send data anymore
	// if server recv trailer, we only need to close recv but still can send data
	if side == clientTransport {
		// for client stream, if trailer frame is received, finish the lifecycle for compatibility
		s.closeSend(nil)
		return s.close(exception, false, side)
	}
	err = s.close(exception, false, side)
	return err
}

func (s *stream) onReadRstFrame(fr *Frame, side sideType) (err error) {
	var rstEx error
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
	// todo: retrieve endpoint information and hop_index
	if side == clientTransport {
		rstEx = errDownstreamCancel.NewBuilder().WithSide(side).WithCauseAndTypeId(ex, ex.TypeId())
	} else {
		rstEx = errUpstreamCancel.NewBuilder().WithSide(side).WithCauseAndTypeId(err, ex.TypeId())
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
	err = s.close(rstEx, false, side)
	return err
}

func unmarshalApplicationException(buf []byte) (*thrift.ApplicationException, error) {
	_, msgType, _, i, err := thrift.Binary.ReadMessageBegin(buf)
	if err != nil {
		return nil, err
	}
	if msgType != thrift.EXCEPTION {
		// todo: create a string map
		return nil, fmt.Errorf("thrift message type want Exception, but got %d", msgType)
	}
	buf = buf[i:]

	ex := thrift.NewApplicationException(thrift.UNKNOWN_APPLICATION_EXCEPTION, "")
	_, err = ex.FastRead(buf)
	if err != nil {
		return nil, err
	}
	return ex, nil
}
