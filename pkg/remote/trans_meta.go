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
)

// MetaHandler reads or writes metadata through certain protocol.
type MetaHandler interface {
	WriteMeta(ctx context.Context, msg Message) (context.Context, error)
	ReadMeta(ctx context.Context, msg Message) (context.Context, error)
}

// StreamingMetaHandler reads or writes metadata through streaming header(http2 header)
type StreamingMetaHandler interface {
	// OnConnectStream writes metadata before create a stream
	OnConnectStream(ctx context.Context) (context.Context, error)
	// OnReadStream reads metadata before read first message from stream
	OnReadStream(ctx context.Context) (context.Context, error)
}

// FrameMetaHandler deals with MetaFrame, HeaderFrame and TrailerFrame
// It's used by TTHeader Streaming
type FrameMetaHandler interface {
	// ReadMeta is called after a MetaFrame is received by client
	// MetaFrame is not necessary and a typical scenario is sent by a proxy which takes over service discovery,
	// By sending the MetaFrame before server's HeaderFrame, proxy can inform the client about the real address.
	ReadMeta(ctx context.Context, message Message) error
	// ClientReadHeader is called after a HeaderFrame is received by client
	ClientReadHeader(ctx context.Context, message Message) error
	// ServerReadHeader is called after a HeaderFrame is received by server
	// The context returned will be used to replace the stream context
	ServerReadHeader(ctx context.Context, message Message) (context.Context, error)
	// WriteHeader is called before a HeaderFrame is sent by client/server
	WriteHeader(ctx context.Context, message Message) error
	// ReadTrailer is called after a TrailerFrame is received by client/server
	ReadTrailer(ctx context.Context, message Message) error
	// WriteTrailer is called before a TrailerFrame is sent by client/server
	WriteTrailer(ctx context.Context, message Message) error
}
