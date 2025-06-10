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

package grpcframe

import (
	"encoding/binary"
	"strings"
	"testing"

	"github.com/cloudwego/gopkg/bufiox"

	"github.com/cloudwego/gopkg/protocol/thrift"
	"golang.org/x/net/http2"

	"github.com/cloudwego/kitex/internal/test"
)

type mockNetpollReader struct {
	bufiox.Reader
	buf []byte
}

func (r *mockNetpollReader) Next(n int) (p []byte, err error) {
	return r.buf[:n], nil
}

func getThriftApplicationExceptionBytes() []byte {
	ex := thrift.NewApplicationException(1, "test thrift ApplicationException")
	buf, _ := thrift.MarshalFastMsg("test", thrift.EXCEPTION, 1, ex)
	return buf
}

func getHTTP2FrameHeaderBytes() []byte {
	fh := http2.FrameHeader{
		Length: 1,
		Type:   http2.FrameData,
	}
	buf := make([]byte, frameHeaderLen)
	buf[0] = byte(fh.Length >> 16)
	buf[1] = byte(fh.Length >> 8)
	buf[2] = byte(fh.Length)
	buf[3] = byte(fh.Type)
	buf[4] = byte(fh.Flags)
	binary.BigEndian.PutUint32(buf[5:], fh.StreamID)
	return buf
}

func Test_Framer_readAndCheckFrameHeader(t *testing.T) {
	// mock netpoll.Reader
	fr := &Framer{}
	http2MaxFrameLen := uint32(16384)
	fr.SetMaxReadFrameSize(http2MaxFrameLen)
	testcases := []struct {
		desc                   string
		reader                 bufiox.Reader
		checkFrameHeaderAndErr func(*testing.T, http2.FrameHeader, error)
	}{
		{
			desc: "thrift ApplicationException",
			reader: &mockNetpollReader{
				buf: getThriftApplicationExceptionBytes(),
			},
			checkFrameHeaderAndErr: func(t *testing.T, header http2.FrameHeader, err error) {
				test.Assert(t, err != nil, err)
				t.Log(err)
				test.Assert(t, strings.Contains(err.Error(), http2.ErrFrameTooLarge.Error()), err)
				test.Assert(t, strings.Contains(err.Error(), "invalid frame"), err)
			},
		},
		{
			desc: "normal HTTP2 FrameHeader",
			reader: &mockNetpollReader{
				buf: getHTTP2FrameHeaderBytes(),
			},
			checkFrameHeaderAndErr: func(t *testing.T, header http2.FrameHeader, err error) {
				test.Assert(t, err == nil, err)
				test.Assert(t, header.Length == 1, header)
				test.Assert(t, header.Type == http2.FrameData, header)
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.desc, func(t *testing.T) {
			fr.reader = tc.reader
			fh, err := fr.readAndCheckFrameHeader()
			tc.checkFrameHeaderAndErr(t, fh, err)
		})
	}
}
