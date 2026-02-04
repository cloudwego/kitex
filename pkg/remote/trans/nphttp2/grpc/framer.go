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
	"io"
	"net"

	"github.com/bytedance/gopkg/lang/dirtmake"
	"github.com/bytedance/gopkg/lang/mcache"
	"github.com/cloudwego/netpoll"
	"golang.org/x/net/http2/hpack"

	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/grpc/grpcframe"
)

type framer struct {
	*grpcframe.Framer
	reader netpoll.Reader
	writer bufWriter
}

func newFramer(conn net.Conn, writeBufferSize, readBufferSize, maxHeaderListSize uint32, reuseCfg ReuseWriteBufferConfig) *framer {
	var r netpoll.Reader
	if npConn, ok := conn.(interface{ Reader() netpoll.Reader }); ok {
		r = npConn.Reader()
	} else {
		r = netpoll.NewReader(conn)
	}
	w := newBufWriter(conn, int(writeBufferSize), reuseCfg)
	fr := &framer{
		reader: r,
		writer: w,
		Framer: grpcframe.NewFramer(w, r),
	}
	fr.SetMaxReadFrameSize(http2MaxFrameLen)
	// Opt-in to Frame reuse API on framer to reduce garbage.
	// Frames aren't safe to read from after a subsequent call to ReadFrame.
	fr.SetReuseFrames()
	fr.MaxHeaderListSize = maxHeaderListSize
	fr.ReadMetaHeaders = hpack.NewDecoder(http2InitHeaderTableSize, nil)
	return fr
}

type ReuseWriteBufferConfig struct {
	Enable bool
}

type bufWriter interface {
	Write(b []byte) (n int, err error)
	Flush() error
	GetOffset() int
}

func newBufWriter(writer io.Writer, batchSize int, reuseCfg ReuseWriteBufferConfig) bufWriter {
	w := &keepBufWriter{
		batchSize: batchSize,
		writer:    writer,
	}

	if !reuseCfg.Enable {
		// Using pre-allocated memory dedicated to each connection
		w.buf = dirtmake.Bytes(batchSize*2, batchSize*2)
		return w
	}

	return &reuseBufWriter{w}
}

// keepBufWriter pre-allocates batchSize * 2 buf and keeps it resident throughout the connection lifecycle.
type keepBufWriter struct {
	writer    io.Writer
	buf       []byte
	offset    int
	batchSize int
	err       error
}

func (w *keepBufWriter) Write(b []byte) (n int, err error) {
	if w.err != nil {
		return 0, w.err
	}
	if w.batchSize == 0 { // buffer has been disabled.
		return w.writer.Write(b)
	}
	for len(b) > 0 {
		nn := copy(w.buf[w.offset:], b)
		b = b[nn:]
		w.offset += nn
		n += nn
		if w.offset >= w.batchSize {
			if err = w.flushAllocatedBuffer(); err != nil {
				return n, err
			}
		}
	}
	return n, err
}

func (w *keepBufWriter) Flush() error {
	return w.flushAllocatedBuffer()
}

func (w *keepBufWriter) GetOffset() int {
	return w.offset
}

func (w *keepBufWriter) flushAllocatedBuffer() error {
	if w.err != nil {
		return w.err
	}
	if w.offset == 0 {
		return nil
	}
	_, w.err = w.writer.Write(w.buf[:w.offset])
	w.offset = 0
	return w.err
}

// During the Write=>Write=> … =>Write=>Flush cycle,
// the first Write allocates batchSize * 2 buf from mcache, and the final Flush returns it.
type reuseBufWriter struct {
	*keepBufWriter
}

func (w *reuseBufWriter) Write(b []byte) (n int, err error) {
	if w.err != nil {
		return 0, w.err
	}
	if w.batchSize == 0 { // buffer has been disabled.
		return w.writer.Write(b)
	}
	if w.buf == nil {
		w.buf = mcache.Malloc(w.batchSize*2, w.batchSize*2)
	}
	for len(b) > 0 {
		nn := copy(w.buf[w.offset:], b)
		b = b[nn:]
		w.offset += nn
		n += nn
		if w.offset >= w.batchSize {
			if err = w.flushAllocatedBuffer(); err != nil {
				return n, err
			}
		}
	}
	return n, err
}

func (w *reuseBufWriter) Flush() error {
	err := w.flushAllocatedBuffer()
	// Reuse only when err is nil, to prevent the underlying connection from still holding buf after a Write failure.
	if err == nil && w.buf != nil {
		mcache.Free(w.buf)
		w.buf = nil
	}
	return err
}

func (w *reuseBufWriter) GetOffset() int {
	return w.offset
}
