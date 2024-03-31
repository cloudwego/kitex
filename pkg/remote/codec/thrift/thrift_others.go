//go:build (!amd64 && !arm64) || windows || !go1.16 || go1.23 || disablefrugal
// +build !amd64,!arm64 windows !go1.16 go1.23 disablefrugal

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

package thrift

import (
	"github.com/cloudwego/kitex/pkg/remote"
)

// hyperMarshalEnabled indicates that if there are high priority message codec for current platform.
func (c thriftCodec) hyperMarshalEnabled() bool {
	return false
}

// hyperMarshalAvailable indicates that if high priority message codec is available.
func hyperMarshalAvailable(data interface{}) bool {
	return false
}

// hyperMessageUnmarshalEnabled indicates that if there are high priority message codec for current platform.
func (c thriftCodec) hyperMessageUnmarshalEnabled() bool {
	return false
}

// hyperMessageUnmarshalAvailable indicates that if high priority message codec is available.
func (c thriftCodec) hyperMessageUnmarshalAvailable(data interface{}, payloadLen int) bool {
	return false
}

func (c thriftCodec) hyperMarshal(out remote.ByteBuffer, methodName string, msgType remote.MessageType, seqID int32, data interface{}) error {
	panic("unreachable code")
}

func (c thriftCodec) hyperMarshalBody(data interface{}) (buf []byte, err error) {
	panic("unreachable code")
}

func (c thriftCodec) hyperMessageUnmarshal(buf []byte, data interface{}) error {
	panic("unreachable code")
}
