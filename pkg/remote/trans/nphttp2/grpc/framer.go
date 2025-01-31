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

	"github.com/cloudwego/gopkg/bufiox"

	"github.com/bytedance/gopkg/lang/dirtmake"
	"github.com/cloudwego/netpoll"
	"golang.org/x/net/http2/hpack"

	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/grpc/grpcframe"
)

type framer struct {
	*grpcframe.Framer
	reader bufiox.Reader
	writer *bufWriter
}

func newFramer(conn net.Conn, writeBufferSize, readBufferSize, maxHeaderListSize uint32) *framer {
	var r bufiox.Reader
	// Initialize a bufiox.Reader based on the connection:
	// 1. If the connection's reader is a `bufiox.Reader`, use it directly.
	// 2. If the connection's reader is a `netpoll.Reader`, wrap it in a `netpollBufioxReader`.
	// 3. Otherwise, create a `bufiox.DefaultReader` with the connection.
	if bc, ok := conn.(interface{ Reader() bufiox.Reader }); ok {
		r = bc.Reader()
	} else {
		if npConn, ok := conn.(interface{ Reader() netpoll.Reader }); ok {
			r = &netpollBufioxReader{Reader: npConn.Reader()}
		} else {
			r = bufiox.NewDefaultReader(conn)
		}
	}
	w := newBufWriter(conn, int(writeBufferSize))
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

type bufWriter struct {
	writer    io.Writer
	buf       []byte
	offset    int
	batchSize int
	err       error
}

func newBufWriter(writer io.Writer, batchSize int) *bufWriter {
	return &bufWriter{
		buf:       dirtmake.Bytes(batchSize*2, batchSize*2),
		batchSize: batchSize,
		writer:    writer,
	}
}

func (w *bufWriter) Write(b []byte) (n int, err error) {
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
			err = w.Flush()
		}
	}
	return n, err
}

func (w *bufWriter) Flush() error {
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

var _ bufiox.Reader = &netpollBufioxReader{}

// netpollBufioxReader implements bufiox.Reader with netpoll.Reader
type netpollBufioxReader struct {
	netpoll.Reader
	readLen int
}

func (r *netpollBufioxReader) Next(n int) (p []byte, err error) {
	p, err = r.Reader.Next(n)
	if err != nil {
		return nil, err
	}
	r.readLen += len(p)
	return p, nil
}

func (r *netpollBufioxReader) ReadBinary(bs []byte) (n int, err error) {
	p, err := r.Next(len(bs))
	if err != nil {
		return 0, err
	}
	n = copy(bs, p)
	return n, nil
}

func (r *netpollBufioxReader) Skip(n int) (err error) {
	err = r.Reader.Skip(n)
	if err != nil {
		return err
	}
	r.readLen += n
	return nil
}

func (r *netpollBufioxReader) ReadLen() (n int) {
	return r.readLen
}

func (r *netpollBufioxReader) Release(e error) (err error) {
	r.readLen = 0
	return r.Reader.Release()
}
