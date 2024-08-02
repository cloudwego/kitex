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

package server

// Invoker is for calling handler function wrapped by Kitex suites without connection.

import (
	"context"
	"errors"

	internal_server "github.com/cloudwego/kitex/internal/server"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/bound"
	"github.com/cloudwego/kitex/pkg/remote/trans/invoke"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
)

// InvokeCaller is the abstraction for invoker call.
type InvokeCaller interface {
	Call(invoke.Message) error
}

// Invoker is the abstraction for invoker.
type Invoker interface {
	RegisterService(svcInfo *serviceinfo.ServiceInfo, handler interface{}, opts ...RegisterOption) error
	Init() (err error)
	InvokeCaller
}

type tInvoker struct {
	*server

	h invoke.Handler
}

// invokerMetaDecoder is used to update `PayloadLen` of `remote.Message`.
// It fixes kitex returning err when apache codec is not available due to msg.PayloadLen() == 0.
// Because users may not add transport header like transport.Framed
// to invoke.Message when calling msg.SetRequestBytes.
// This is NOT expected and it's caused by kitex design fault.
type invokerMetaDecoder struct {
	remote.Codec

	d remote.MetaDecoder
}

func (d *invokerMetaDecoder) DecodeMeta(ctx context.Context, msg remote.Message, in remote.ByteBuffer) error {
	err := d.d.DecodeMeta(ctx, msg, in)
	if err != nil {
		return err
	}
	// cool ... no need to do anything.
	// added transport header?
	if msg.PayloadLen() > 0 {
		return nil
	}
	// use the whole buffer
	// coz for invoker remote.ByteBuffer always contains the whole msg payload
	if n := in.ReadableLen(); n > 0 {
		msg.SetPayloadLen(n)
	}
	return nil
}

func (d *invokerMetaDecoder) DecodePayload(ctx context.Context, msg remote.Message, in remote.ByteBuffer) error {
	return d.d.DecodePayload(ctx, msg, in)
}

// NewInvoker creates new Invoker.
func NewInvoker(opts ...Option) Invoker {
	s := &server{
		opt:  internal_server.NewOptions(opts),
		svcs: newServices(),
	}
	if codec, ok := s.opt.RemoteOpt.Codec.(remote.MetaDecoder); ok {
		// see comment on type `invokerMetaDecoder`
		s.opt.RemoteOpt.Codec = &invokerMetaDecoder{
			Codec: s.opt.RemoteOpt.Codec,
			d:     codec,
		}
	}
	s.init()
	return &tInvoker{
		server: s,
	}
}

// Init does initialization job for invoker.
func (s *tInvoker) Init() (err error) {
	if len(s.server.svcs.svcMap) == 0 {
		return errors.New("run: no service. Use RegisterService to set one")
	}
	s.initBasicRemoteOption()
	// for server trans info handler
	if len(s.server.opt.MetaHandlers) > 0 {
		transInfoHdlr := bound.NewTransMetaHandler(s.server.opt.MetaHandlers)
		doAddBoundHandler(transInfoHdlr, s.server.opt.RemoteOpt)
	}
	s.Lock()
	s.h, err = s.newInvokeHandler()
	s.Unlock()
	if err != nil {
		return err
	}
	for i := range onServerStart {
		go onServerStart[i]()
	}
	return nil
}

// Call implements the InvokeCaller interface.
func (s *tInvoker) Call(msg invoke.Message) error {
	return s.h.Call(msg)
}

func (s *tInvoker) newInvokeHandler() (handler invoke.Handler, err error) {
	opt := s.server.opt.RemoteOpt
	tf := invoke.NewIvkTransHandlerFactory()
	hdlr, err := tf.NewTransHandler(opt)
	if err != nil {
		return nil, err
	}
	hdlr.(remote.InvokeHandleFuncSetter).SetInvokeHandleFunc(s.eps)
	pl := remote.NewTransPipeline(hdlr)
	hdlr.SetPipeline(pl)
	for _, ib := range opt.Inbounds {
		pl.AddInboundHandler(ib)
	}
	for _, ob := range opt.Outbounds {
		pl.AddOutboundHandler(ob)
	}
	return invoke.NewIvkHandler(opt, pl)
}
