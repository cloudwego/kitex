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
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cloudwego/gopkg/protocol/thrift"
	"github.com/cloudwego/gopkg/protocol/ttheader"

	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/streaming"
	"github.com/cloudwego/kitex/pkg/transmeta"
)

var _ ClientStreamMeta = (*clientStream)(nil)

func newClientStream(ctx context.Context, writer streamWriter, smeta streamFrame) *clientStream {
	s := newBasicStream(ctx, writer, smeta)
	cs := &clientStream{
		stream:     s,
		headerSig:  make(chan int32, 1),
		trailerSig: make(chan int32, 1),
	}
	s.reader = newStreamReaderWithCtxDoneCallback(cs.ctxDoneCallback)
	return cs
}

// initial state: streamStateActive
//
//                     close()
// streamStateActive ----------> streamStateInactive
//       |                                 ^
//       |                                 |
//       | CloseSend()                     | close()
//       v                                 |
//       +--- streamStateHalfCloseLocal ---+

type clientStream struct {
	*stream
	state            int32
	metaFrameHandler MetaFrameHandler
	// closeStreamException ensures that Send could also get the same
	// exception as Recv
	closeStreamException atomic.Value // type must be of *Exception
	storeExceptionOnce   sync.Once
	streamTimeout        time.Duration // whole stream timeout

	// for Header()/Trailer()
	headerSig  chan int32
	trailerSig chan int32
}

func (s *clientStream) Header() (streaming.Header, error) {
	sig := <-s.headerSig
	switch sig {
	case streamSigNone:
		return make(streaming.Header), nil
	case streamSigActive:
		return s.header, nil
	case streamSigInactive:
		return nil, ErrClosedStream
	case streamSigCancel:
		return nil, ErrCanceledStream
	}
	return nil, errors.New("invalid stream signal")
}

func (s *clientStream) Trailer() (streaming.Trailer, error) {
	sig := <-s.trailerSig
	switch sig {
	case streamSigNone:
		return make(streaming.Trailer), nil
	case streamSigActive:
		return s.trailer, nil
	case streamSigInactive:
		return nil, ErrClosedStream
	case streamSigCancel:
		return nil, ErrCanceledStream
	}
	return nil, errors.New("invalid stream signal")
}

func (s *clientStream) SendMsg(ctx context.Context, req any) error {
	if state := atomic.LoadInt32(&s.state); state != streamStateActive {
		if ex := s.closeStreamException.Load(); ex == nil {
			return errIllegalOperation.newBuilder().withSide(clientSide).withCause(errors.New("stream is closed send"))
		} else {
			return ex.(error)
		}
	}
	return s.stream.SendMsg(ctx, req)
}

func (s *clientStream) RecvMsg(ctx context.Context, resp any) error {
	return s.stream.RecvMsg(ctx, resp)
}

// CloseSend by clientStream only send trailer frame and will not close the stream
func (s *clientStream) CloseSend(ctx context.Context) error {
	if !atomic.CompareAndSwapInt32(&s.state, streamStateActive, streamStateHalfCloseLocal) {
		return nil
	}
	return s.sendTrailer(nil)
}

func (s *clientStream) Context() context.Context {
	return s.ctx
}

func (s *clientStream) Cancel(err error) {
	finalEx, cancelPath := s.parseCancelErr(err)
	s.close(finalEx, true, cancelPath)
}

// ctxDoneCallback convert ctx.Err() to ttstream related err and close client-side stream.
// it is invoked in container.Pipe
func (s *clientStream) ctxDoneCallback(ctx context.Context) {
	finalEx, cancelPath := s.parseCtxErr(ctx)

	s.close(finalEx, true, cancelPath)
}

func (s *clientStream) parseCancelErr(err error) (finalEx *Exception, cancelPath string) {
	svcName := s.rpcInfo.From().ServiceName()
	switch err {
	// biz code invokes cancel()
	case nil, context.Canceled:
		finalEx = errBizCancel.newBuilder().withSide(clientSide)
		// the initial node sending rst, the original cancelPath is empty
		cancelPath = appendCancelPath("", svcName)
	// stream timeout
	case context.DeadlineExceeded:
		finalEx = newTimeoutException(errStreamTimeout, clientSide, s.streamTimeout)
		cancelPath = appendCancelPath("", svcName)
	default:
		if tEx, ok := err.(*Exception); ok {
			// for cascading cancel case, we need to change the side from server to client
			finalEx = tEx.newBuilder().withSide(clientSide)
			cancelPath = appendCancelPath(tEx.cancelPath, svcName)
		} else {
			// ctx provided by other sources(e.g. gRPC handler has been canceled, cErr is gRPC error)
			finalEx = errInternalCancel.newBuilder().withSide(clientSide).withCause(err)
			// as upstream cascading path may have existed(e.g. gRPC service chains), using non-ttstream path
			// as a unified placeholder enables quick identification of such scenarios
			cancelPath = appendCancelPath("non-ttstream path", svcName)
		}
	}
	return
}

// parseCtxErr parses information in ctx.Err and returning ttstream Exception and cascading cancelPath
func (s *clientStream) parseCtxErr(ctx context.Context) (finalEx *Exception, cancelPath string) {
	return s.parseCancelErr(ctx.Err())
}

func (s *clientStream) close(exception error, sendRst bool, cancelPath string) {
	if exception != nil {
		// store exception before change clientStream state
		// otherwise clientStream.Send would not get the real stream closed reason
		// only store once
		s.storeExceptionOnce.Do(func() {
			s.closeStreamException.Store(exception)
		})
	}
	oldState := atomic.SwapInt32(&s.state, streamStateInactive)
	if oldState == streamStateInactive {
		// stream has been closed before
		return
	}
	select {
	case s.headerSig <- streamSigInactive:
	default:
	}
	select {
	case s.trailerSig <- streamSigInactive:
	default:
	}
	// clientStream.Recv would get the exception
	s.reader.close(exception)
	if sendRst {
		if err := s.sendRst(exception, cancelPath); err != nil {
			klog.Errorf("KITEX: stream[%d] send Rst Frame failed, err: %v", s.sid, err)
		}
	}
	// ServerStreaming would invoke CloseSend automatically
	if oldState != streamStateHalfCloseLocal {
		// For clientStream, if trailer frame or rst frame are received, finish the lifecycle directly.
		// But Some downstream components may not be upgraded and need to receive a Trailer Frame to end the Stream's lifecycle.
		// For better compatibility, we choose to send a Trailer Frame when the clientStream is closed.
		//
		// todo: remove this logic in the future.
		s.sendTrailer(nil)
	}
	s.runCloseCallback(exception)
}

func (s *clientStream) setMetaFrameHandler(metaHandler MetaFrameHandler) {
	s.metaFrameHandler = metaHandler
}

func (s *clientStream) setStreamTimeout(tm time.Duration) {
	s.streamTimeout = tm
}

// === clientStream OnRead callback

func (s *clientStream) onReadMetaFrame(fr *Frame) error {
	if s.metaFrameHandler == nil {
		return nil
	}
	return s.metaFrameHandler.OnMetaFrame(s.ctx, fr.meta, fr.header, fr.payload)
}

func (s *clientStream) onReadHeaderFrame(fr *Frame) error {
	if s.header != nil {
		return errUnexpectedHeader.newBuilder().withSide(clientSide).withCause(fmt.Errorf("stream[%d] already set header", s.sid))
	}
	s.header = fr.header
	select {
	case s.headerSig <- streamSigActive:
	default:
		return errUnexpectedHeader.newBuilder().withSide(clientSide).withCause(fmt.Errorf("stream[%d] already set header", s.sid))
	}
	return nil
}

func (s *clientStream) onReadTrailerFrame(fr *Frame) error {
	var exception error
	// when server-side returns non-biz error, it will be wrapped as ApplicationException stored in trailer frame payload
	if len(fr.payload) > 0 {
		// exception is type of (*thrift.ApplicationException)
		appEx, err := decodeException(fr.payload)
		if err != nil {
			exception = errIllegalFrame.newBuilder().withSide(clientSide).withCause(err)
		} else {
			exception = errApplicationException.newBuilder().withSide(clientSide).withCause(appEx)
		}
	} else if len(fr.trailer) > 0 {
		// when server-side returns biz error, payload is empty and biz error information is stored in trailer frame header
		bizErr, err := transmeta.ParseBizStatusErr(fr.trailer)
		if err != nil {
			exception = errIllegalBizErr.newBuilder().withSide(clientSide).withCause(err)
		} else if bizErr != nil {
			// todo: unify bizErr with Exception
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

	// client-side stream recv trailer, the lifecycle of whole stream has ended
	s.close(exception, false, "")
	return nil
}

func (s *clientStream) onReadRstFrame(fr *Frame) (err error) {
	var rstEx *Exception
	var appEx *thrift.ApplicationException
	if len(fr.payload) > 0 {
		// exception is type of (*thrift.ApplicationException)
		appEx, err = decodeException(fr.payload)
		if err != nil {
			klog.Errorf("KITEX: stream[%d] unmarshal Exception in rst frame failed, err: %v", s.sid, err)
			appEx = defaultRstException
		}
	} else {
		klog.Errorf("KITEX: stream[%d] recv rst frame without payload", s.sid)
		appEx = defaultRstException
	}

	// distract cancelPath information
	var cancelPath string
	if fr.header != nil {
		hdrVia, ok := fr.header[ttheader.HeaderTTStreamCancelPath]
		if ok {
			cancelPath = hdrVia
		}
	}
	rstEx = errDownstreamCancel.newBuilder().withSide(clientSide)
	if cancelPath != "" {
		rstEx = rstEx.setOrAppendCancelPath(cancelPath).withCauseAndTypeId(appEx, appEx.TypeId())
	} else {
		rstEx = rstEx.withCauseAndTypeId(appEx, appEx.TypeId())
	}

	select {
	case s.trailerSig <- streamSigCancel:
	default:
	}
	select {
	case s.headerSig <- streamSigCancel:
	default:
	}

	// when receiving rst frame, we should close stream and there is no need to send rst frame
	s.close(rstEx, false, "")
	return nil
}
