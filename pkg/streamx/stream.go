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

package streamx

import (
	"context"
)

/* Streaming Mode
---------------  [Unary Streaming]  ---------------
--------------- (Req) returns (Res) ---------------
client.Send(req)   === req ==>   server.Recv(req)
client.Recv(res)   <== res ===   server.Send(res)


------------------- [Client Streaming] -------------------
--------------- (stream Req) returns (Res) ---------------
client.Send(req)         === req ==>       server.Recv(req)
                             ...
client.Send(req)         === req ==>       server.Recv(req)

client.CloseSend()       === EOF ==>       server.Recv(EOF)
client.Recv(res)         <== res ===       server.SendAndClose(res)
** OR
client.CloseAndRecv(res) === EOF ==>       server.Recv(EOF)
                         <== res ===       server.SendAndClose(res)


------------------- [Server Streaming] -------------------
---------- (Request) returns (stream Response) ----------
client.Send(req)   === req ==>   server.Recv(req)
client.CloseSend() === EOF ==>   server.Recv(EOF)
client.Recv(res)   <== res ===   server.Send(req)
                       ...
client.Recv(res)   <== res ===   server.Send(req)
client.Recv(EOF)   <== EOF ===   server handler return


----------- [Bidirectional Streaming] -----------
--- (stream Request) returns (stream Response) ---
* goroutine 1 *
client.Send(req)   === req ==>   server.Recv(req)
                       ...
client.Send(req)   === req ==>   server.Recv(req)
client.CloseSend() === EOF ==>   server.Recv(EOF)

* goroutine 2 *
client.Recv(res)   <== res ===   server.Send(req)
                       ...
client.Recv(res)   <== res ===   server.Send(req)
client.Recv(EOF)   <== EOF ===   server handler return
*/

type (
	Header  map[string]string
	Trailer map[string]string
)

// Stream define stream APIs
type Stream interface {
	SendMsg(ctx context.Context, m any) error
	RecvMsg(ctx context.Context, m any) error
}

// ClientStream define client stream APIs
type ClientStream interface {
	Stream
	Header() (Header, error)
	Trailer() (Trailer, error)
	CloseSend(ctx context.Context) error
}

// ServerStream define server stream APIs
type ServerStream interface {
	Stream
	SetHeader(hd Header) error
	SendHeader(hd Header) error
	SetTrailer(hd Trailer) error
}

// ServerStreamingClient define client side server streaming APIs
type ServerStreamingClient[Res any] interface {
	Recv(ctx context.Context) (*Res, error)
	ClientStream
}

// ServerStreamingServer define server side server streaming APIs
type ServerStreamingServer[Res any] interface {
	Send(ctx context.Context, res *Res) error
	ServerStream
}

// ClientStreamingClient define client side client streaming APIs
type ClientStreamingClient[Req, Res any] interface {
	Send(ctx context.Context, req *Req) error
	CloseAndRecv(ctx context.Context) (*Res, error)
	ClientStream
}

// ClientStreamingServer define server side client streaming APIs
type ClientStreamingServer[Req, Res any] interface {
	Recv(ctx context.Context) (*Req, error)
	SendAndClose(ctx context.Context, res *Res) error
	ServerStream
}

// BidiStreamingClient define client side bidi streaming APIs
type BidiStreamingClient[Req, Res any] interface {
	Send(ctx context.Context, req *Req) error
	Recv(ctx context.Context) (*Res, error)
	ClientStream
}

// BidiStreamingServer define server side bidi streaming APIs
type BidiStreamingServer[Req, Res any] interface {
	Recv(ctx context.Context) (*Req, error)
	Send(ctx context.Context, res *Res) error
	ServerStream
}
