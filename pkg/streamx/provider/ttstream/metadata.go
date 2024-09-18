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
	"errors"

	"github.com/cloudwego/kitex/pkg/streamx"
)

var ErrInvalidStreamKind = errors.New("invalid stream kind")

// only for meta frame handler
type IntHeader map[uint16]string

// ClientStreamMeta cannot send header directly, should send from ctx
type ClientStreamMeta interface {
	streamx.ClientStream
	Header() (streamx.Header, error)
	Trailer() (streamx.Trailer, error)
}

// ServerStreamMeta cannot read header directly, should read from ctx
type ServerStreamMeta interface {
	streamx.ServerStream
	SetHeader(hd streamx.Header) error
	SendHeader(hd streamx.Header) error
	SetTrailer(hd streamx.Trailer) error
}
