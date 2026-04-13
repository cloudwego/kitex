/*
 * Copyright 2026 CloudWeGo Authors
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

package grpcframe

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"golang.org/x/net/http2"

	"github.com/cloudwego/kitex/internal/test"
)

// newReadBack returns a golang.org/x/net/http2.Framer that reads the bytes
// produced by the grpcframe.Framer. Using the upstream framer as an
// independent parser validates byte-level protocol correctness.
func newReadBack(buf *bytes.Buffer) *http2.Framer {
	return http2.NewFramer(io.Discard, buf)
}

// errWriter always returns an error on Write, used to test write-failure paths.
type errWriter struct{ err error }

func (w *errWriter) Write([]byte) (int, error) { return 0, w.err }

func TestFramer_Write(t *testing.T) {
	testcases := []struct {
		desc      string
		setFramer func(framer *Framer)
	}{
		{
			desc: "without reusing write buffer",
		},
		{
			desc: "with reusing write buffer",
			setFramer: func(framer *Framer) {
				framer.SetWriteBufferPoolEnabled(true)
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.desc, func(t *testing.T) {
			t.Run("WriteData round trip", func(t *testing.T) {
				var buf bytes.Buffer
				fr := NewFramer(&buf, nil)
				if tc.setFramer != nil {
					tc.setFramer(fr)
				}

				payload1 := []byte("hello")
				payload2 := []byte("world frame")
				test.Assert(t, fr.WriteData(1, false, payload1) == nil)
				test.Assert(t, fr.WriteData(3, true, payload2) == nil)

				rb := newReadBack(&buf)

				f1, err := rb.ReadFrame()
				test.Assert(t, err == nil, err)
				d1 := f1.(*http2.DataFrame)
				test.Assert(t, d1.StreamID == 1, d1.StreamID)
				test.Assert(t, !d1.StreamEnded())
				test.Assert(t, bytes.Equal(d1.Data(), payload1), d1.Data())

				f2, err := rb.ReadFrame()
				test.Assert(t, err == nil, err)
				d2 := f2.(*http2.DataFrame)
				test.Assert(t, d2.StreamID == 3, d2.StreamID)
				test.Assert(t, d2.StreamEnded())
				test.Assert(t, bytes.Equal(d2.Data(), payload2), d2.Data())

				// And the buffer must be fully consumed — no trailing bytes.
				test.Assert(t, buf.Len() == 0, buf.Len())
			})
			t.Run("WriteHeaders round trip", func(t *testing.T) {
				var buf bytes.Buffer
				fr := NewFramer(&buf, nil)
				if tc.setFramer != nil {
					tc.setFramer(fr)
				}

				hfp := http2.HeadersFrameParam{
					StreamID:      1,
					BlockFragment: []byte{0x00, 0x01, 0x02, 0x03},
					EndStream:     true,
					EndHeaders:    true,
				}
				test.Assert(t, fr.WriteHeaders(hfp) == nil)

				rb := newReadBack(&buf)
				f, err := rb.ReadFrame()
				test.Assert(t, err == nil, err)
				h := f.(*http2.HeadersFrame)
				test.Assert(t, h.StreamID == 1, h.StreamID)
				test.Assert(t, h.StreamEnded())
				test.Assert(t, h.HeadersEnded())
				test.Assert(t, bytes.Equal(h.HeaderBlockFragment(), hfp.BlockFragment))
			})
			t.Run("WriteSettingsAck zero payload", func(t *testing.T) {
				var buf bytes.Buffer
				fr := NewFramer(&buf, nil)
				if tc.setFramer != nil {
					tc.setFramer(fr)
				}

				test.Assert(t, fr.WriteSettingsAck() == nil)

				rb := newReadBack(&buf)
				f, err := rb.ReadFrame()
				test.Assert(t, err == nil, err)
				sf := f.(*http2.SettingsFrame)
				test.Assert(t, sf.IsAck())
				test.Assert(t, sf.NumSettings() == 0, sf.NumSettings())
				test.Assert(t, buf.Len() == 0, buf.Len())
			})
			t.Run("WriteWindowUpdate zero payload body", func(t *testing.T) {
				var buf bytes.Buffer
				fr := NewFramer(&buf, nil)
				if tc.setFramer != nil {
					tc.setFramer(fr)
				}

				test.Assert(t, fr.WriteWindowUpdate(0, 12345) == nil)

				rb := newReadBack(&buf)
				f, err := rb.ReadFrame()
				test.Assert(t, err == nil, err)
				wuf := f.(*http2.WindowUpdateFrame)
				test.Assert(t, wuf.StreamID == 0)
				test.Assert(t, wuf.Increment == 12345, wuf.Increment)
				test.Assert(t, buf.Len() == 0, buf.Len())
			})
			t.Run("WriteData large payload", func(t *testing.T) {
				var buf bytes.Buffer
				fr := NewFramer(&buf, nil)
				if tc.setFramer != nil {
					tc.setFramer(fr)
				}

				// 16384 bytes = http2 default max DATA frame size
				payload := make([]byte, 16384)
				for i := range payload {
					payload[i] = byte(i % 251)
				}
				test.Assert(t, fr.WriteData(1, true, payload) == nil)

				rb := newReadBack(&buf)
				f, err := rb.ReadFrame()
				test.Assert(t, err == nil, err)
				d := f.(*http2.DataFrame)
				test.Assert(t, d.StreamID == 1)
				test.Assert(t, d.StreamEnded())
				test.Assert(t, bytes.Equal(d.Data(), payload))
				test.Assert(t, buf.Len() == 0, buf.Len())
			})
			t.Run("write failure returns error", func(t *testing.T) {
				writeErr := errors.New("connection reset")
				fr := NewFramer(&errWriter{err: writeErr}, nil)
				if tc.setFramer != nil {
					tc.setFramer(fr)
				}

				err := fr.WriteData(1, false, []byte("data"))
				test.Assert(t, err != nil)
				test.Assert(t, errors.Is(err, writeErr), err)
			})
			t.Run("write failure with zero payload", func(t *testing.T) {
				writeErr := errors.New("broken pipe")
				fr := NewFramer(&errWriter{err: writeErr}, nil)
				if tc.setFramer != nil {
					tc.setFramer(fr)
				}

				err := fr.WriteSettingsAck()
				test.Assert(t, err != nil)
				test.Assert(t, errors.Is(err, writeErr), err)
			})
		})
	}
}
