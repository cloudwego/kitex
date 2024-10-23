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

package remote

import (
	"context"
	"net"

	"github.com/cloudwego/kitex/pkg/streaming"
)

type ServerPipelineHandler interface {
	OnActive(ctx context.Context, conn net.Conn) (context.Context, error)
	OnInactive(ctx context.Context, conn net.Conn) context.Context
	OnRead(ctx context.Context, conn net.Conn) (context.Context, error)
	OnStream(ctx context.Context, st streaming.ServerStream) (context.Context, error)
	OnStreamRead(ctx context.Context, msg Message) (nctx context.Context, err error)
	OnStreamWrite(ctx context.Context, send Message) (nctx context.Context, err error)
}

type ClientPipelineHandler interface {
	OnInactive(ctx context.Context, conn net.Conn) context.Context
	OnConnectStream(ctx context.Context) (nctx context.Context, err error)
	OnStreamRead(ctx context.Context, msg Message) (nctx context.Context, err error)
	OnStreamWrite(ctx context.Context, send Message) (nctx context.Context, err error)
}

// ServerTransPipeline contains TransHandlers.
type ServerTransPipeline struct {
	ServerTransHandler

	svrHdrls []ServerPipelineHandler
}

var _ ServerTransHandler = &ServerTransPipeline{}

func newServerTransPipeline() *ServerTransPipeline {
	return &ServerTransPipeline{}
}

// NewServerTransPipeline is used to create a new TransPipeline.
func NewServerTransPipeline(netHdlr ServerTransHandler) *ServerTransPipeline {
	transPl := newServerTransPipeline()
	transPl.ServerTransHandler = netHdlr
	netHdlr.SetPipeline(transPl)
	return transPl
}

// AddHandler adds an ServerPipelineHandler to the pipeline.
func (p *ServerTransPipeline) AddHandler(hdlr ServerPipelineHandler) *ServerTransPipeline {
	p.svrHdrls = append(p.svrHdrls, hdlr)
	return p
}

// OnActive implements the InboundHandler interface.
func (p *ServerTransPipeline) OnActive(ctx context.Context, conn net.Conn) (context.Context, error) {
	var err error
	for _, h := range p.svrHdrls {
		ctx, err = h.OnActive(ctx, conn)
		if err != nil {
			return ctx, err
		}
	}
	if netHdlr, ok := p.ServerTransHandler.(ServerTransHandler); ok {
		return netHdlr.OnActive(ctx, conn)
	}
	return ctx, nil
}

// OnInactive implements the InboundHandler interface.
func (p *ServerTransPipeline) OnInactive(ctx context.Context, conn net.Conn) {
	for _, h := range p.svrHdrls {
		ctx = h.OnInactive(ctx, conn)
	}
	p.ServerTransHandler.OnInactive(ctx, conn)
}

// OnRead implements the InboundHandler interface.
func (p *ServerTransPipeline) OnRead(ctx context.Context, conn net.Conn) error {
	var err error
	for _, h := range p.svrHdrls {
		ctx, err = h.OnRead(ctx, conn)
		if err != nil {
			return err
		}
	}
	return p.ServerTransHandler.OnRead(ctx, conn)
}

func (p *ServerTransPipeline) OnStream(ctx context.Context, st streaming.ServerStream) error {
	var err error
	for _, h := range p.svrHdrls {
		ctx, err = h.OnStream(ctx, st)
		if err != nil {
			return err
		}
	}
	return p.ServerTransHandler.OnStream(ctx, st)
}

// GracefulShutdown implements the GracefulShutdown interface.
func (p *ServerTransPipeline) GracefulShutdown(ctx context.Context) error {
	if g, ok := p.ServerTransHandler.(GracefulShutdown); ok {
		return g.GracefulShutdown(ctx)
	}
	return nil
}

// ClientTransPipeline contains TransHandlers.
type ClientTransPipeline struct {
	ClientTransHandler

	cliHdrls []ClientPipelineHandler
}

var _ ClientTransHandler = &ClientTransPipeline{}

func newClientTransPipeline() *ClientTransPipeline {
	return &ClientTransPipeline{}
}

func NewClientTransPipeline(netHdlr ClientTransHandler) *ClientTransPipeline {
	transPl := newClientTransPipeline()
	transPl.ClientTransHandler = netHdlr
	netHdlr.SetPipeline(transPl)
	return transPl
}

// AddHandler adds an ClientPipelineHandler to the pipeline.
func (p *ClientTransPipeline) AddHandler(hdlr ClientPipelineHandler) *ClientTransPipeline {
	p.cliHdrls = append(p.cliHdrls, hdlr)
	return p
}

func (p *ClientTransPipeline) OnInactive(ctx context.Context, conn net.Conn) {
	for _, h := range p.cliHdrls {
		ctx = h.OnInactive(ctx, conn)
	}
	p.ClientTransHandler.OnInactive(ctx, conn)
}

func (p *ClientTransPipeline) NewStream(ctx context.Context, conn net.Conn) (st streaming.ClientStream, err error) {
	for _, h := range p.cliHdrls {
		ctx, err = h.OnConnectStream(ctx)
		if err != nil {
			return nil, err
		}
	}
	return p.ClientTransHandler.NewStream(ctx, conn)
}

func NewServerStreamPipeline(st ServerStream, pipeline *ServerTransPipeline) *ServerStreamPipeline {
	return &ServerStreamPipeline{ServerStream: st, pipeline: pipeline}
}

type ServerStreamPipeline struct {
	ServerStream

	pipeline *ServerTransPipeline
}

func (p *ServerStreamPipeline) Write(ctx context.Context, sendMsg Message) (nctx context.Context, err error) {
	for _, h := range p.pipeline.svrHdrls {
		ctx, err = h.OnStreamWrite(ctx, sendMsg)
		if err != nil {
			return ctx, err
		}
	}
	return p.ServerStream.Write(ctx, sendMsg)
}

func (p *ServerStreamPipeline) Read(ctx context.Context, msg Message) (context.Context, error) {
	var err error
	ctx, err = p.ServerStream.Read(ctx, msg)
	if err != nil {
		return ctx, err
	}
	for _, h := range p.pipeline.svrHdrls {
		ctx, err = h.OnStreamRead(ctx, msg)
		if err != nil {
			return ctx, err
		}
	}
	return ctx, nil
}

func NewClientStreamPipeline(st ClientStream, pipeline *ClientTransPipeline) *ClientStreamPipeline {
	return &ClientStreamPipeline{ClientStream: st, pipeline: pipeline}
}

type ClientStreamPipeline struct {
	ClientStream

	pipeline *ClientTransPipeline
}

func (p *ClientStreamPipeline) Write(ctx context.Context, sendMsg Message) (nctx context.Context, err error) {
	for _, h := range p.pipeline.cliHdrls {
		ctx, err = h.OnStreamWrite(ctx, sendMsg)
		if err != nil {
			return ctx, err
		}
	}
	return p.ClientStream.Write(ctx, sendMsg)
}

func (p *ClientStreamPipeline) Read(ctx context.Context, msg Message) (context.Context, error) {
	var err error
	ctx, err = p.ClientStream.Read(ctx, msg)
	if err != nil {
		return ctx, err
	}
	for _, h := range p.pipeline.cliHdrls {
		ctx, err = h.OnStreamRead(ctx, msg)
		if err != nil {
			return ctx, err
		}
	}
	return ctx, nil
}
