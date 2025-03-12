/*
 * Copyright 2025 CloudWeGo Authors
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

package pbtpl

import (
	"fmt"
	"io"
	"strings"
)

// Args ...
type Args struct {
	Services []*Service
	StreamX  bool
}

// Service ...
type Service struct {
	Name    string
	Methods []*Method
}

// Method ...
type Method struct {
	Name    string
	ReqType string
	ResType string

	ClientStream bool
	ServerStream bool
}

// Render uses the given args to render kitex grpc code.
func Render(w io.Writer, in *Args) {
	fm := func(format string, aa ...any) {
		fmt.Fprintf(w, format, aa...)
		fmt.Fprintln(w)
	}
	for _, s := range in.Services {
		renderService(fm, s, in.StreamX)
	}
}

func renderService(fm func(format string, aa ...any), s *Service, streamx bool) {
	// interface of the given service
	fm("\ntype %s interface {", s.Name)
	for _, m := range s.Methods {
		switch {
		case m.ClientStream: // client or bid bidirectional streaming
			if streamx {
				fm("%s(ctx context.Context, stream %s_%sServer) (err error)",
					m.Name, s.Name, m.Name)
			} else {
				fm("%s(stream %s_%sServer) (err error)",
					m.Name, s.Name, m.Name)
			}

		case m.ServerStream:
			if streamx {
				fm("%s(ctx context.Context, req %s, stream %s_%sServer) (err error)",
					m.Name, m.ReqType, s.Name, m.Name)
			} else {
				fm("%s(req %s, stream %s_%sServer) (err error)",
					m.Name, m.ReqType, s.Name, m.Name)
			}

		default: // unary
			fm("%s(ctx context.Context, req %s) (res %s, err error)", m.Name, m.ReqType, m.ResType)
		}
	}
	fm("}")

	// types used by the interface
	for _, m := range s.Methods {
		if !m.ClientStream && !m.ServerStream {
			continue
		}

		if streamx { // use generics if StreamX
			switch {
			case m.ClientStream && m.ServerStream:
				fm("\ntype %s_%sServer = streaming.BidiStreamingServer[%s, %s]",
					s.Name, m.Name, notPtr(m.ReqType), notPtr(m.ResType))
			case m.ClientStream:
				fm("\ntype %s_%sServer = streaming.ClientStreamingServer[%s, %s]",
					s.Name, m.Name, notPtr(m.ReqType), notPtr(m.ResType))
			case m.ServerStream:
				fm("\ntype %s_%sServer streaming.ServerStreamingServer[%s]",
					s.Name, m.Name, notPtr(m.ResType))
			}
			continue
		}

		fm("\ntype %s_%sServer interface {", s.Name, m.Name)
		fm("streaming.Stream")
		switch {
		case m.ClientStream && m.ServerStream:
			fm("Recv() (%s, error)", m.ReqType)
			fm("Send(%s) error", m.ResType)

		case m.ClientStream:
			fm("Recv() (%s, error)", m.ReqType)
			fm("SendAndClose(%s) error", m.ResType)

		case m.ServerStream:
			fm("Send(%s) error", m.ResType)

		}
		fm("}")
	}
}

func notPtr(s string) string {
	return strings.TrimLeft(strings.TrimSpace(s), "*")
}
