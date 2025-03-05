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
	"context"
	"net"
	"testing"
	"time"

	"github.com/cloudwego/kitex/internal/test"
)

func TestControlBuf(t *testing.T) {
	ctx := context.Background()
	cb := newControlBuffer(ctx.Done())

	// test put()
	testItem := &ping{}
	err := cb.put(testItem)
	test.Assert(t, err == nil, err)

	// test get() with block
	item, err := cb.get(true)
	test.Assert(t, err == nil, err)
	test.Assert(t, item == testItem, err)

	// test get() with no block
	item, err = cb.get(false)
	test.Assert(t, err == nil, err)
	test.Assert(t, item == nil, err)

	// test executeAndPut()
	success, err := cb.execute(func(it interface{}) bool {
		return false
	}, &ping{})

	test.Assert(t, err == nil, err)
	test.Assert(t, !success, err)

	// test throttle() mock a lot of response frame so throttle() will block current goroutine
	for i := 0; i < maxQueuedTransportResponseFrames+5; i++ {
		err := cb.put(&ping{})
		test.Assert(t, err == nil, err)
	}

	// start a new goroutine to consume response frame
	go func() {
		time.Sleep(time.Millisecond * 100)
		for {
			it, err := cb.get(false)
			if err != nil || it == nil {
				break
			}
		}
	}()

	cb.throttle()

	// test finish()
	cb.finish(ErrConnClosing)
}

type mockNetConn struct {
	net.Conn
}

func (m *mockNetConn) Write(b []byte) (int, error) {
	return len(b), nil
}

func Test_LoopyWriter_Send_DataFrame(t *testing.T) {
	var err error
	ch := make(chan struct{})
	cb := newControlBuffer(ch)
	framer := newFramer(&mockNetConn{}, defaultWriteBufferSize, defaultReadBufferSize, defaultClientMaxHeaderListSize)
	bdp := &bdpEstimator{
		bdp:               initialWindowSize,
		updateFlowControl: func(n uint32) {},
	}
	writer := newLoopyWriter(clientSide, framer, cb, bdp)

	streamId := uint32(1)
	done := make(chan struct{})
	wq := newWriteQuota(defaultWriteQuota, done)
	hdrFrame := &headerFrame{
		streamID: streamId,
		initStream: func(u uint32) error {
			return nil
		},
		wq: wq,
	}
	// initialize stream
	err = writer.handle(hdrFrame)
	test.Assert(t, err == nil, err)
	// end Data Frame with EOS flag
	endFrame := newDataFrame()
	endFrame.endStream = true
	endFrame.streamID = 1
	defer endFrame.Release()
	err = writer.handle(endFrame)
	test.Assert(t, err == nil, err)
	// normal Data Frame after end Data Frame
	normalFrame := newDataFrame()
	normalFrame.streamID = 1
	normalFrame.h = []byte("test-hdr")
	normalFrame.d = []byte("test-data")
	defer normalFrame.Release()
	err = writer.handle(normalFrame)
	test.Assert(t, err == nil, err)

	isEmpty, err := writer.processData()
	test.Assert(t, err == nil, err)
	test.Assert(t, !isEmpty, isEmpty)
}
