//go:build !windows

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

	"github.com/cloudwego/gopkg/protocol/ttheader"
	"github.com/cloudwego/netpoll"

	"github.com/cloudwego/kitex/pkg/serviceinfo"
	"github.com/cloudwego/kitex/pkg/streaming"
)

type mockStreamWriter struct {
	writeFrameFunc  func(f *Frame) error
	closeStreamFunc func(sid int32) error
}

func (m mockStreamWriter) WriteFrame(f *Frame) error {
	if m.writeFrameFunc != nil {
		return m.writeFrameFunc(f)
	}
	return nil
}

func (m mockStreamWriter) CloseStream(sid int32) error {
	if m.closeStreamFunc != nil {
		return m.closeStreamFunc(sid)
	}
	return nil
}

func newTestStreamPipe(sinfo *serviceinfo.ServiceInfo, method string) (*clientStream, *serverStream, error) {
	cfd, sfd := netpoll.GetSysFdPairs()
	cconn, err := netpoll.NewFDConnection(cfd)
	if err != nil {
		return nil, nil, err
	}
	sconn, err := netpoll.NewFDConnection(sfd)
	if err != nil {
		return nil, nil, err
	}

	intHeader := make(IntHeader)
	strHeader := make(streaming.Header)
	ctrans := newClientTransport(cconn, nil)
	ctx := context.Background()
	cs := newClientStream(ctx, ctrans, streamFrame{sid: genStreamID(), method: method, protocolID: ttheader.ProtocolIDThriftStruct})
	if err = ctrans.WriteStream(ctx, cs, intHeader, strHeader); err != nil {
		return nil, nil, err
	}
	strans := newServerTransport(sconn)
	ss, err := strans.ReadStream(context.Background())
	if err != nil {
		return nil, nil, err
	}

	return cs, ss, nil
}
