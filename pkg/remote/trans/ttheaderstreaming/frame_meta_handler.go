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
	"strconv"
	"time"

	"github.com/bytedance/gopkg/cloud/metainfo"

	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/transmeta"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	header_transmeta "github.com/cloudwego/kitex/pkg/transmeta"
)

var (
	_ remote.FrameMetaHandler = (*clientTTHeaderFrameHandler)(nil)
	_ remote.FrameMetaHandler = (*serverTTHeaderFrameHandler)(nil)
)

type clientTTHeaderFrameHandler struct{}

func NewClientTTHeaderFrameHandler() remote.FrameMetaHandler {
	return &clientTTHeaderFrameHandler{}
}

// ReadMeta does nothing by default (only for client)
func (c *clientTTHeaderFrameHandler) ReadMeta(ctx context.Context, message remote.Message) error {
	return nil
}

// ClientReadHeader does nothing by default at client side
func (c *clientTTHeaderFrameHandler) ClientReadHeader(ctx context.Context, message remote.Message) error {
	return nil
}

// ServerReadHeader should not be called at client side
func (c *clientTTHeaderFrameHandler) ServerReadHeader(ctx context.Context, message remote.Message) (context.Context, error) {
	panic("ServerReadHeader should not be called at client side")
}

// WriteHeader writes necessary keys for TTHeader Streaming
func (c *clientTTHeaderFrameHandler) WriteHeader(ctx context.Context, msg remote.Message) error {
	ri := msg.RPCInfo()

	intInfo := map[uint16]string{
		transmeta.ToMethod: ri.Invocation().MethodName(),
	}
	if deadline, ok := ctx.Deadline(); ok {
		if now := time.Now(); deadline.After(now) {
			intInfo[transmeta.RPCTimeout] = strconv.Itoa(int(deadline.Sub(now).Milliseconds()))
		}
	}
	msg.TransInfo().PutTransIntInfo(intInfo)

	strInfo := msg.TransInfo().TransStrInfo()
	metainfo.SaveMetaInfoToMap(ctx, strInfo)
	strInfo[transmeta.HeaderIDLServiceName] = ri.Invocation().ServiceName()
	return nil
}

func (c *clientTTHeaderFrameHandler) ReadTrailer(ctx context.Context, msg remote.Message) error {
	intInfo := msg.TransInfo().TransIntInfo()
	strInfo := msg.TransInfo().TransStrInfo()

	// TransError
	if err := parseTransErrorIfExists(intInfo); err != nil {
		return err
	}

	// BizStatusError
	if err := header_transmeta.ParseBizStatusErrorToRPCInfo(strInfo, msg.RPCInfo()); err != nil {
		return err
	}

	// TODO: metainfo backward keys
	return nil
}

func parseTransErrorIfExists(intInfo map[uint16]string) error {
	if transCode, exists := intInfo[transmeta.TransCode]; exists {
		if code, err := strconv.Atoi(transCode); err != nil {
			msg := fmt.Sprintf("parse trans code failed, code: %s, err: %s", transCode, err)
			return remote.NewTransErrorWithMsg(remote.InternalError, msg)
		} else if code != 0 {
			transMsg := intInfo[transmeta.TransMessage]
			return remote.NewTransErrorWithMsg(int32(code), transMsg)
		}
	}
	return nil
}

// WriteTrailer for client
func (c *clientTTHeaderFrameHandler) WriteTrailer(ctx context.Context, msg remote.Message) error {
	if err, ok := msg.Data().(error); ok && err != nil {
		injectTransError(msg, err)
	}
	return nil
}

type serverTTHeaderFrameHandler struct{}

func NewServerTTHeaderFrameHandler() remote.FrameMetaHandler {
	return &serverTTHeaderFrameHandler{}
}

// ReadMeta should not be called at server side
func (s *serverTTHeaderFrameHandler) ReadMeta(ctx context.Context, message remote.Message) error {
	panic("ReadMeta should not be called at server side")
}

// ClientReadHeader should not be called at server side
func (s *serverTTHeaderFrameHandler) ClientReadHeader(ctx context.Context, message remote.Message) error {
	panic("ClientReadHeader should not be called at server side")
}

// ServerReadHeader reads necessary keys for TTHeader Streaming
func (s *serverTTHeaderFrameHandler) ServerReadHeader(ctx context.Context, msg remote.Message) (context.Context, error) {
	ri := msg.RPCInfo()
	transInfo := msg.TransInfo()
	intInfo := transInfo.TransIntInfo()

	if cfg := rpcinfo.AsMutableRPCConfig(ri.Config()); cfg != nil {
		cfg.SetPayloadCodec(msg.ProtocolInfo().CodecType)
		timeout := intInfo[transmeta.RPCTimeout]
		if timeoutMS, err := strconv.Atoi(timeout); err == nil {
			_ = cfg.SetRPCTimeout(time.Duration(timeoutMS) * time.Millisecond)
		}
	}

	ink := ri.Invocation().(rpcinfo.InvocationSetter)
	if method, exists := intInfo[transmeta.ToMethod]; !exists { // method must exist in Header Frame
		ErrHeaderFrameInvalidToMethod := errors.New("missing method in ttheader streaming header frame")
		return ctx, ErrHeaderFrameInvalidToMethod
	} else {
		ink.SetMethodName(method)
	}

	if idlServiceName, exists := transInfo.TransStrInfo()[transmeta.HeaderIDLServiceName]; exists {
		// IDL Service Name is only necessary for multi-service servers.
		ink.SetServiceName(idlServiceName)
	}

	// cloudwego metainfo
	ctx = metainfo.SetMetaInfoFromMap(ctx, msg.TransInfo().TransStrInfo())

	// grpc style metadata
	return injectGRPCMetadata(ctx, msg.TransInfo().TransStrInfo())
}

// WriteHeader does nothing by default at server side
func (s *serverTTHeaderFrameHandler) WriteHeader(ctx context.Context, msg remote.Message) error {
	return nil
}

// ReadTrailer for server
func (s *serverTTHeaderFrameHandler) ReadTrailer(ctx context.Context, msg remote.Message) error {
	intInfo := msg.TransInfo().TransIntInfo()

	// TransError from Client
	if err := parseTransErrorIfExists(intInfo); err != nil {
		return err
	}

	return nil
}

// WriteTrailer ...
func (s *serverTTHeaderFrameHandler) WriteTrailer(ctx context.Context, msg remote.Message) error {
	// TODO: metainfo backward keys
	if err, ok := msg.Data().(error); ok && err != nil {
		injectTransError(msg, err)
		return nil // no need to deal with BizStatusError
	}
	header_transmeta.InjectBizStatusError(msg.TransInfo().TransStrInfo(), msg.RPCInfo().Invocation().BizStatusErr())
	return nil
}

// injectTransError set an error to message header
// if err is not a remote.TransError, the exceptionType will be set to remote.InternalError
func injectTransError(message remote.Message, err error) {
	exceptionType := remote.InternalError
	var transErr *remote.TransError
	if errors.As(err, &transErr) {
		exceptionType = int(transErr.TypeID())
	}
	message.TransInfo().PutTransIntInfo(map[uint16]string{
		transmeta.TransCode:    strconv.Itoa(int(exceptionType)),
		transmeta.TransMessage: err.Error(),
	})
}
