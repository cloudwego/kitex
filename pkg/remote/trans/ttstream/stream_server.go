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

	"github.com/cloudwego/gopkg/protocol/thrift"
	"github.com/cloudwego/gopkg/protocol/ttheader"

	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/streaming"
	"github.com/cloudwego/kitex/pkg/transmeta"
)

var _ ServerStreamMeta = (*serverStream)(nil)

func newServerStream(ctx context.Context, writer streamWriter, smeta streamFrame) *serverStream {
	s := newBasicStream(ctx, writer, smeta)
	ss := &serverStream{stream: s}
	s.reader = newStreamReaderWithCtxDoneCallback(ss.ctxDoneCallback)
	return ss
}

// initial state: streamStateActive
//
//                     close()
// streamStateActive ----------> streamStateInactive
//       |                                  ^
//       |                                  |
//       | closeRecv()                      | close()
//       v                                  |
//       +--- streamStateHalfCloseRemote ---+

type serverStream struct {
	*stream
	state      int32
	cancelFunc cancelWithReason
	timeout    time.Duration
}

func (s *serverStream) SetHeader(hd streaming.Header) error {
	return s.writeHeader(hd)
}

func (s *serverStream) SendHeader(hd streaming.Header) error {
	if err := s.writeHeader(hd); err != nil {
		return err
	}
	return s.sendHeader()
}

// writeHeader copy kvs into s.wheader
func (s *serverStream) writeHeader(hd streaming.Header) error {
	if s.wheader == nil {
		return fmt.Errorf("stream header already sent")
	}
	for k, v := range hd {
		s.wheader[k] = v
	}
	return nil
}

// sendHeader send header to peer
func (s *serverStream) sendHeader() (err error) {
	wheader := s.wheader
	s.wheader = nil
	if wheader == nil {
		return fmt.Errorf("stream header already sent")
	}
	err = s.writeFrame(headerFrameType, wheader, nil, nil)
	return err
}

func (s *serverStream) SetTrailer(tl streaming.Trailer) error {
	return s.writeTrailer(tl)
}

// writeTrailer write trailer to peer
func (s *serverStream) writeTrailer(tl streaming.Trailer) (err error) {
	if s.wtrailer == nil {
		return fmt.Errorf("stream trailer already sent")
	}
	for k, v := range tl {
		s.wtrailer[k] = v
	}
	return nil
}

func (s *serverStream) RecvMsg(ctx context.Context, req any) error {
	return s.stream.RecvMsg(ctx, req)
}

// SendMsg should send left header first
func (s *serverStream) SendMsg(ctx context.Context, res any) error {
	if st := atomic.LoadInt32(&s.state); st == streamStateInactive {
		return s.ctx.Err()
	}
	if s.wheader != nil {
		if err := s.sendHeader(); err != nil {
			return err
		}
	}
	return s.stream.SendMsg(ctx, res)
}

// CloseSend by serverStream will be called after server handler returned
// after CloseSend stream cannot be access again
func (s *serverStream) CloseSend(exception error) error {
	s.close(errBizHandlerReturnCancel)
	return s.sendTrailer(exception)
}

// closeRecv called only when server receiving Trailer Frame
func (s *serverStream) closeRecv(exception error) error {
	// client->server data transferring has stopped
	atomic.CompareAndSwapInt32(&s.state, streamStateActive, streamStateHalfCloseRemote)
	s.reader.close(exception)
	return nil
}

func (s *serverStream) close(exception *Exception) error {
	// support cascading cancel
	// we must cancel the ctx first before changing the state
	// otherwise Recv/Send will not get the expected exception
	s.cancelFunc(exception)
	if atomic.SwapInt32(&s.state, streamStateInactive) == streamStateInactive {
		return nil
	}

	s.reader.close(exception)
	s.runCloseCallback()

	return nil
}

// === serverStream onRead callback

func (s *serverStream) onReadTrailerFrame(fr *Frame) (err error) {
	var exception error
	// when server-side returns non-biz error, it will be wrapped as ApplicationException stored in trailer frame payload
	if len(fr.payload) > 0 {
		// exception is type of (*thrift.ApplicationException)
		_, _, err = thrift.UnmarshalFastMsg(fr.payload, nil)
		exception = ErrApplicationException.newBuilder().withSide(serverSide).withCause(err)
	} else if len(fr.trailer) > 0 {
		// when server-side returns biz error, payload is empty and biz error information is stored in trailer frame header
		bizErr, err := transmeta.ParseBizStatusErr(fr.trailer)
		if err != nil {
			exception = errIllegalBizErr.newBuilder().withSide(serverSide).withCause(err)
		} else if bizErr != nil {
			// bizErr is independent of rpc exception handling
			exception = bizErr
		}
	}
	s.trailer = fr.trailer

	// server-side stream recv trailer, we only need to close recv but still can send data
	return s.closeRecv(exception)
}

func (s *serverStream) onReadRstFrame(fr *Frame) (err error) {
	// parse ApplicationException in payload
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
		klog.Infof("KITEX: stream[%d] recv rst frame without payload", s.sid)
		appEx = defaultRstException
	}

	// parse cancelPath information
	var cancelPath string
	if fr.header != nil {
		// cascading cancel path
		hdrCp, ok := fr.header[ttheader.HeaderTTStreamCancelPath]
		if ok {
			cancelPath = hdrCp
		}
	}

	// build Exception
	rstEx = errUpstreamCancel.newBuilder().withSide(serverSide)
	if cancelPath != "" {
		rstEx = rstEx.setOrAppendCancelPath(cancelPath).withCauseAndTypeId(appEx, appEx.TypeId())
	} else {
		rstEx = rstEx.withCauseAndTypeId(appEx, appEx.TypeId())
	}

	// when receiving rst frame, we should close stream and there is no need to send rst frame
	return s.close(rstEx)
}

// closeTest is only used in unit tests for mocking Proxy Egress/Ingress send Rst Frame to downstream and upstream
func (s *serverStream) closeTest(exception error, cancelPath string) error {
	// support cascading cancel
	// we must cancel the ctx first before changing the state
	// otherwise Recv/Send will not get the expected exception
	s.cancelFunc(errInternalCancel.newBuilder().withCause(exception))
	if atomic.SwapInt32(&s.state, streamStateInactive) == streamStateInactive {
		return nil
	}

	s.reader.close(exception)
	if err := s.sendRst(exception, cancelPath); err != nil {
		return err
	}
	s.runCloseCallback()
	return nil
}

func (s *serverStream) ctxDoneCallback(ctx context.Context) {
	finalEx := s.parseCtxErr(ctx)
	s.close(finalEx)
}

func (s *serverStream) parseCtxErr(ctx context.Context) (finalEx *Exception) {
	cErr := ctx.Err()
	switch cErr {
	// stream timeout
	case context.DeadlineExceeded:
		finalEx = newTimeoutException(errStreamTimeout, serverSide, s.timeout)
	// other close stream scenarios, there is no need to process
	default:
		if ex, ok := cErr.(*Exception); ok {
			finalEx = ex
		} else {
			finalEx = errInternalCancel.newBuilder().withSide(serverSide).withCause(cErr)
		}
	}

	return finalEx
}
