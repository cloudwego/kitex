/*
 * Copyright 2022 CloudWeGo Authors
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

package grpc

import (
	"bytes"
	"sync"
)

// fixme 关联太大，无法通过修改大小写完成

// MockNewServerSideStream mock a stream with recvBuffer
func MockNewServerSideStream() *Stream {
	recvBuffer := newRecvBuffer()
	trReader := &transportReader{
		reader: &recvBufferReader{
			ctx:     nil,
			ctxDone: nil,
			recv:    recvBuffer,
			freeBuffer: func(buffer *bytes.Buffer) {
				buffer.Reset()
			},
		},
		windowHandler: func(i int) {
		},
	}

	stream := &Stream{
		id:           1,
		st:           nil,
		ct:           nil,
		ctx:          nil,
		cancel:       nil,
		done:         nil,
		ctxDone:      nil,
		method:       "",
		recvCompress: "",
		sendCompress: "",
		buf:          recvBuffer,
		trReader:     trReader,
		fc:           nil,
		wq:           newWriteQuota(defaultWriteQuota, nil),
		requestRead: func(i int) {
		},
		HeaderChan:       nil,
		headerChanClosed: 0,
		headerValid:      false,
		hdrMu:            sync.Mutex{},
		header:           nil,
		trailer:          nil,
		noHeaders:        false,
		headerSent:       0,
		state:            0,
		status:           nil,
		bytesReceived:    0,
		unprocessed:      0,
		contentSubtype:   "",
	}

	return stream
}
