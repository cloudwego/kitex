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

// Package thrift provides thrift idl parser and codec for generic call
package thrift

import (
	"context"

	"github.com/cloudwego/gopkg/bufiox"
	"github.com/cloudwego/gopkg/protocol/thrift/base"
)

const (
	structWrapLen = 4
)

// MessageReader read from thrift.TProtocol with method
type MessageReader interface {
	Read(ctx context.Context, method string, isClient bool, dataLen int, in bufiox.Reader) (interface{}, error)
}

// MessageWriter write to thrift.TProtocol
type MessageWriter interface {
	Write(ctx context.Context, out bufiox.Writer, msg interface{}, method string, isClient bool, requestBase *base.Base) error
}
