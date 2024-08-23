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

package ttheaderstreaming

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"runtime/debug"
	"sync/atomic"

	"github.com/cloudwego/kitex/pkg/gofunc"
	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/codec"
	"github.com/cloudwego/kitex/pkg/remote/trans"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/metadata"
	remote_transmeta "github.com/cloudwego/kitex/pkg/remote/transmeta"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/streaming"
	"github.com/cloudwego/kitex/transport"
)

const (
	defaultSendQueueSize = 64
)

var (
	_ streaming.Stream = (*ttheaderStream)(nil)

	ErrIllegalHeaderWrite = errors.New("ttheader streaming: the stream is done or headers was already sent")
	ErrInvalidFrame       = errors.New("ttheader streaming: invalid frame")
	ErrServerGotMetaFrame = errors.New("ttheader streaming: server got metadata frame")
	ErrSendClosed         = errors.New("ttheader streaming: send closed")
	ErrDirtyFrame         = errors.New("ttheader streaming: dirty frame")

	idAllocator = uint64(0)

	sendQueueSize = defaultSendQueueSize
)

// SetSendQueueSize sets the send queue size.
func SetSendQueueSize(size int) {
	sendQueueSize = size
}

type sendRequest struct {
	message      remote.Message
	finishSignal chan error
}

func (r sendRequest) waitFinishSignal(ctx context.Context) error {
	select {
	case err := <-r.finishSignal:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

type ttheaderStream struct {
	id      uint64
	ctx     context.Context
	cancel  context.CancelFunc
	conn    net.Conn
	ri      rpcinfo.RPCInfo
	codec   remote.Codec
	rpcRole remote.RPCRole
	ext     trans.Extension

	recvState     state
	lastRecvError error

	sendState             state
	sendQueue             chan sendRequest
	sendLoopCloseSignal   chan struct{}
	dirtyFrameCloseSignal chan struct{}

	grpcMetadata grpcMetadata
	frameHandler remote.FrameMetaHandler
}

func newTTHeaderStream(
	ctx context.Context, conn net.Conn, ri rpcinfo.RPCInfo, codec remote.Codec,
	role remote.RPCRole, ext trans.Extension, handler remote.FrameMetaHandler,
) *ttheaderStream {
	ctx, cancel := context.WithCancel(ctx)
	st := &ttheaderStream{
		id:           atomic.AddUint64(&idAllocator, 1),
		ctx:          ctx,
		cancel:       cancel,
		conn:         conn,
		ri:           ri,
		codec:        codec,
		rpcRole:      role,
		ext:          ext,
		frameHandler: handler,

		sendQueue:             make(chan sendRequest, sendQueueSize),
		sendLoopCloseSignal:   make(chan struct{}),
		dirtyFrameCloseSignal: make(chan struct{}),
	}
	gofunc.GoFunc(ctx, func() {
		// server will replace st.ctx, so sendLoop should not read st.ctx directly, to avoid concurrent read/write
		st.sendLoop(ctx)
	})
	return st
}

func (t *ttheaderStream) Context() context.Context {
	return t.ctx
}

func (t *ttheaderStream) SetHeader(md metadata.MD) error {
	if t.rpcRole != remote.Server {
		panic("this method should only be used in server side stream!")
	}
	if t.sendState.hasHeader() || t.sendState.isClosed() {
		return ErrIllegalHeaderWrite
	}
	t.grpcMetadata.setHeader(md)
	return nil
}

func (t *ttheaderStream) SendHeader(md metadata.MD) error {
	if t.rpcRole != remote.Server {
		panic("this method should only be used in server side stream!")
	}
	t.grpcMetadata.setHeader(md)
	return t.sendHeaderFrame(nil)
}

func (t *ttheaderStream) SetTrailer(md metadata.MD) {
	if t.rpcRole != remote.Server {
		panic("this method should only be used in server side stream!")
	}
	if md.Len() == 0 || t.sendState.isClosed() {
		return
	}
	t.grpcMetadata.setTrailer(md)
}

func (t *ttheaderStream) Header() (metadata.MD, error) {
	if t.rpcRole != remote.Client {
		panic("Header() should only be used in client side stream!")
	}
	if old := t.recvState.setHeader(); old != received {
		if err := t.readUntilTargetFrame(nil, codec.FrameTypeHeader); err != nil {
			return nil, err
		}
	}
	return t.grpcMetadata.headersReceived, nil
}

func (t *ttheaderStream) Trailer() metadata.MD {
	if t.rpcRole != remote.Client {
		panic("Trailer() should only be used in client side stream!")
	}
	if old := t.recvState.setTrailer(); old != received {
		_ = t.readUntilTargetFrame(nil, codec.FrameTypeTrailer)
	}
	return t.grpcMetadata.trailerReceived
}

func (t *ttheaderStream) RecvMsg(m interface{}) error {
	if t.recvState.isClosed() {
		return t.lastRecvError
	}
	readErr := t.readUntilTargetFrame(m, codec.FrameTypeData)
	if readErr == io.EOF && t.ri.Invocation().BizStatusErr() != nil {
		return nil // same behavior as grpc streaming: return nil for biz status error
	}
	return readErr
}

func (t *ttheaderStream) SendMsg(m interface{}) error {
	if t.sendState.isClosed() {
		return ErrSendClosed
	}
	if err := t.sendHeaderFrame(nil); err != nil {
		return err
	}

	message := t.newMessageToSend(m, codec.FrameTypeData)
	return t.putMessageToSendQueue(message)
}

func (t *ttheaderStream) Close() error {
	return t.closeSend(nil)
}

func (t *ttheaderStream) waitSendLoopClose() {
	<-t.sendLoopCloseSignal
}

func (t *ttheaderStream) sendHeaderFrame(msg remote.Message) error {
	if old := t.sendState.setHeader(); old == sent {
		return nil
	}
	if msg == nil { // client header is constructed in client_handler.go
		msg = t.newMessageToSend(nil, codec.FrameTypeHeader)
		if err := t.frameHandler.WriteHeader(t.ctx, msg); err != nil {
			return fmt.Errorf("server frameMetaHandler.WriteHeader failed: %w", err)
		}
	}
	if err := t.grpcMetadata.injectHeader(msg); err != nil {
		return fmt.Errorf("grpcMetadata.injectHeader failed: err=%w", err)
	}
	return t.putMessageToSendQueue(msg)
}

func (t *ttheaderStream) sendTrailerFrame(invokeErr error) error {
	if old := t.sendState.setTrailer(); old == sent {
		return nil
	}
	if err := t.sendHeaderFrame(nil); err != nil { // make sure header has been sent
		return err
	}
	message := t.newTrailerFrame(invokeErr)
	return t.putMessageToSendQueue(message)
}

func (t *ttheaderStream) closeRecv(err error) {
	if old := t.recvState.setClosed(); old == closed {
		return
	}
	if err == nil {
		err = io.EOF
	}
	t.lastRecvError = err
}

func (t *ttheaderStream) newMessageToSend(m interface{}, frameType string) remote.Message {
	message := remote.NewMessage(m, nil, t.ri, remote.Stream, t.rpcRole)
	message.SetProtocolInfo(remote.NewProtocolInfo(transport.TTHeader, t.ri.Config().PayloadCodec()))
	message.Tags()[codec.HeaderFlagsKey] = codec.HeaderFlagsStreaming
	message.TransInfo().TransIntInfo()[remote_transmeta.FrameType] = frameType
	return message
}

func (t *ttheaderStream) readMessageWatchingCtxDone(data interface{}) (message remote.Message, err error) {
	frameReceived := make(chan struct{}, 1)

	var readErr error
	gofunc.GoFunc(t.ctx, func() {
		message, readErr = t.readMessage(data)
		frameReceived <- struct{}{}
	})

	select {
	case <-frameReceived:
		return message, readErr
	case <-t.ctx.Done():
		doneErr := t.ctx.Err()
		t.closeRecv(doneErr)
		return nil, doneErr
	}
}

func (t *ttheaderStream) readMessage(data interface{}) (message remote.Message, err error) {
	defer func() {
		if panicInfo := recover(); panicInfo != nil {
			err = fmt.Errorf("readMessage panic: %v, stack=%s", panicInfo, string(debug.Stack()))
		}
		if err != nil {
			t.closeRecv(err)
			if message != nil {
				message.Recycle()
				message = nil
			}
		}
	}()
	if t.recvState.isClosed() {
		return nil, t.lastRecvError
	}
	message = remote.NewMessage(data, nil, t.ri, remote.Stream, t.rpcRole)
	bufReader := t.ext.NewReadByteBuffer(t.ctx, t.conn, message)
	err = t.codec.Decode(t.ctx, message, bufReader)
	return message, err
}

func (t *ttheaderStream) parseMetaFrame(message remote.Message) (err error) {
	if oldState := t.recvState.setMeta(); oldState == received {
		return fmt.Errorf("unexpected meta frame, recvState=%v", t.recvState)
	}
	if err = t.frameHandler.ReadMeta(t.ctx, message); err != nil {
		return fmt.Errorf("clientMetaHandler.ReadMeta failed: %w", err)
	}
	return err
}

func (t *ttheaderStream) parseHeaderFrame(message remote.Message) (err error) {
	t.recvState.setHeader()
	if t.rpcRole == remote.Client {
		if err = t.frameHandler.ClientReadHeader(t.ctx, message); err != nil {
			return fmt.Errorf("frameHandler.ClientReadHeader failed: %w", err)
		}
		return t.grpcMetadata.parseHeader(message)
	} else /* remote.Server */ {
		// server header frame is already processed in server_handler.go
		return nil
	}
}

func (t *ttheaderStream) parseDataFrame(message remote.Message) error {
	if !t.recvState.hasHeader() {
		return fmt.Errorf("unexpected data frame before header frame, recvState=%v", t.recvState)
	}
	return nil
}

func (t *ttheaderStream) parseTrailerFrame(message remote.Message) (err error) {
	defer func() {
		t.closeRecv(err)
	}()

	t.recvState.setTrailer()
	if !t.recvState.hasHeader() {
		return fmt.Errorf("unexpected trailer frame, recvState=%v", t.recvState)
	}

	if err = t.frameHandler.ReadTrailer(t.ctx, message); err != nil {
		var transErr *remote.TransError
		if errors.As(err, &transErr) {
			return transErr
		}
		return fmt.Errorf("frameHandler.ReadTrailer failed: %w", err)
	}

	if t.rpcRole == remote.Client {
		if err = t.grpcMetadata.parseTrailer(message); err != nil {
			return fmt.Errorf("client grpcMetadata.parseTrailer failed: %w", err)
		}
	}
	return io.EOF
}

func (t *ttheaderStream) readAndParseMessage(data interface{}) (string, error) {
	message, err := t.readMessage(data)
	defer func() {
		if message != nil {
			message.Recycle()
		}
	}()
	if err != nil {
		return codec.FrameTypeInvalid, err
	}
	frameType := codec.MessageFrameType(message)
	switch frameType {
	case codec.FrameTypeMeta:
		if err = t.parseMetaFrame(message); err != nil {
			return frameType, err
		}
	case codec.FrameTypeHeader:
		if err = t.parseHeaderFrame(message); err != nil {
			return frameType, err
		}
	case codec.FrameTypeData:
		if err = t.parseDataFrame(message); err != nil {
			return frameType, err
		}
	case codec.FrameTypeTrailer:
		if err = t.parseTrailerFrame(message); err != nil {
			return frameType, err
		}
	default:
		return frameType, ErrInvalidFrame
	}
	return frameType, nil
}

func (t *ttheaderStream) readUntilTargetFrame(data interface{}, targetFrameType string) error {
	frameReceived := make(chan struct{}, 1)

	var readErr error
	gofunc.GoFunc(t.ctx, func() {
		readErr = t.readUntilTargetFrameSynchronously(data, targetFrameType)
		frameReceived <- struct{}{}
	})

	select {
	case <-frameReceived:
		return readErr
	case <-t.ctx.Done():
		doneErr := t.ctx.Err()
		t.closeRecv(doneErr)
		return doneErr
	}
}

func (t *ttheaderStream) readUntilTargetFrameSynchronously(data interface{}, targetFrameType string) (err error) {
	defer func() {
		if err != nil {
			t.closeRecv(err)
		}
	}()
	var frameType string
	for {
		if frameType, err = t.readAndParseMessage(data); err != nil {
			return err // err=io.EOF for TrailerFrame
		}
		if frameType == targetFrameType {
			return nil
		}
	}
}

func (t *ttheaderStream) putMessageToSendQueue(message remote.Message) error {
	sendReq := sendRequest{
		message:      message,
		finishSignal: make(chan error, 1),
	}
	select {
	case t.sendQueue <- sendReq:
		return t.waitSendFinishedSignal(sendReq.finishSignal)
	case <-t.ctx.Done():
		t.sendState.setClosed() // state change is enough here, since sendLoop will deal with the TrailerFrame
		return t.ctx.Err()
	case <-t.sendLoopCloseSignal:
		return ErrSendClosed
	}
}

func (t *ttheaderStream) waitSendFinishedSignal(finishSignal chan error) error {
	select {
	case err := <-finishSignal:
		return err
	case <-t.ctx.Done():
		return t.ctx.Err()
	case <-t.sendLoopCloseSignal:
		return ErrSendClosed
	}
}

func (t *ttheaderStream) closeSend(err error) error {
	if old := t.sendState.setClosed(); old == closed {
		return nil
	}

	// server side has sent trailers, then there is no need for client to send trailer
	if t.rpcRole == remote.Client && t.recvState.isClosed() {
		return nil
	}
	sendErr := t.sendTrailerFrame(err)
	t.waitSendLoopClose() // make sure all frames are sent before the connection is reused
	if t.rpcRole == remote.Server {
		t.closeRecv(ErrSendClosed)
		t.cancel() // release blocking RecvMsg call
	}
	return sendErr
}

func (t *ttheaderStream) newTrailerFrame(err error) remote.Message {
	msg := t.newMessageToSend(err, codec.FrameTypeTrailer)
	if err = t.frameHandler.WriteTrailer(t.ctx, msg); err != nil {
		klog.Warnf("frameHandler.WriteTrailer failed: %v", err)
	}
	if err = t.grpcMetadata.injectTrailer(msg); err != nil {
		klog.Warnf("grpcMetadata.injectTrailer failed: err=%v", err)
	}
	// always send the TrailerFrame regardless of the error
	return msg
}

func (t *ttheaderStream) sendLoop(ctx context.Context) {
	defer func() {
		if panicInfo := recover(); panicInfo != nil {
			err := fmt.Errorf("sendLoop panic: info=%v, stack=%s", panicInfo, string(debug.Stack()))
			klog.CtxWarnf(t.ctx, "KITEX: ttheader streaming sendLoop panic, info=%v, stack=%s",
				panicInfo, string(debug.Stack()))
			t.closeRecv(err)
			t.cancel() // release any possible blocking call
		}
		t.sendState.setClosed()
		t.notifyAllSendRequests()
		close(t.sendLoopCloseSignal)
	}()
	var message remote.Message
	for {
		var finishSignal chan error
		select {
		case req := <-t.sendQueue:
			finishSignal = req.finishSignal
			message = req.message
		case <-ctx.Done():
			message = t.newTrailerFrame(ctx.Err())
		case <-t.dirtyFrameCloseSignal:
			return
		}
		frameType := codec.MessageFrameType(message)
		err := t.writeMessage(message) // message is recycled
		if finishSignal != nil {
			finishSignal <- err
		}
		if err != nil || frameType == codec.FrameTypeTrailer {
			break
		}
	}
}

func (t *ttheaderStream) notifyAllSendRequests() {
	for {
		select {
		case req, ok := <-t.sendQueue:
			if ok {
				req.finishSignal <- ErrSendClosed
			}
		default:
			return
		}
	}
}

func (t *ttheaderStream) writeMessage(message remote.Message) error {
	defer message.Recycle() // not referenced after this, including the HeaderFrame
	out := t.ext.NewWriteByteBuffer(t.ctx, t.conn, message)
	if err := t.codec.Encode(t.ctx, message, out); err != nil {
		return err
	}
	return out.Flush()
}

func (t *ttheaderStream) loadOutgoingMetadataForGRPC() {
	t.grpcMetadata.loadHeadersToSend(t.ctx)
}

func (t *ttheaderStream) clientReadMetaFrame() error {
	if t.recvState.isClosed() {
		return t.lastRecvError
	}
	return t.readUntilTargetFrame(nil, codec.FrameTypeMeta)
}

func (t *ttheaderStream) serverReadHeaderFrame(opt *remote.ServerOption) (nCtx context.Context, err error) {
	var clientHeader remote.Message
	defer func() {
		if clientHeader != nil {
			clientHeader.Recycle()
		}
	}()
	if clientHeader, err = t.readMessageWatchingCtxDone(nil); err != nil {
		return nil, err
	}
	if codec.MessageFrameType(clientHeader) != codec.FrameTypeHeader {
		close(t.dirtyFrameCloseSignal)
		return nil, ErrDirtyFrame
	}

	t.recvState.setHeader()

	if nCtx, err = t.serverReadHeaderMeta(opt, clientHeader); err != nil {
		return nil, err
	}

	t.ctx = opt.TracerCtl.DoStart(nCtx, t.ri) // replace streamCtx since new values are added to the ctx
	return t.ctx, err
}

func (t *ttheaderStream) serverReadHeaderMeta(opt *remote.ServerOption, msg remote.Message) (context.Context, error) {
	var err error
	nCtx := t.ctx // with cancel

	if nCtx, err = t.frameHandler.ServerReadHeader(nCtx, msg); err != nil {
		return nil, fmt.Errorf("frameHandler.ServerReadHeader failed: %w", err)
	}

	// StreamingMetaHandler.OnReadStream
	for _, handler := range opt.StreamingMetaHandlers {
		if nCtx, err = handler.OnReadStream(nCtx); err != nil {
			return nil, err
		}
	}
	return nCtx, nil
}
