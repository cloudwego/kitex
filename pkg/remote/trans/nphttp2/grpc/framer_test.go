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

package grpc

import (
	"io"
	"net"
	"testing"

	"github.com/cloudwego/gopkg/bufiox"
	"github.com/golang/mock/gomock"

	mocksnetpoll "github.com/cloudwego/kitex/internal/mocks/netpoll"
	"github.com/cloudwego/kitex/internal/test"
)

type mockConnWithBufioxReader struct {
	net.Conn
	bufioxReader bufiox.Reader
}

func (c *mockConnWithBufioxReader) Reader() bufiox.Reader {
	return c.bufioxReader
}

func TestNewFramer(t *testing.T) {
	// conn without bufiox reader
	var conn net.Conn
	conn = &mockConn{}
	fr := newFramer(conn, 0, 0, 0)
	_, ok := fr.reader.(*bufiox.DefaultReader)
	test.Assert(t, ok)

	// conn with bufiox reader
	reader := &bufiox.DefaultReader{}
	conn = &mockConnWithBufioxReader{bufioxReader: reader}
	fr = newFramer(conn, 0, 0, 0)
	test.Assert(t, fr.reader == reader)

	// netpoll conn
	conn = &mockNetpollConn{}
	fr = newFramer(conn, 0, 0, 0)
	_, ok = fr.reader.(*netpollBufioxReader)
	test.Assert(t, ok)
}

func TestNetpollBufioxReader(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	var (
		validReadLength = 5
		eofReadLength   = 8
		validSkipLength = 5
		eofSkipLength   = 3
		expectedString  = "Hello"
	)

	mockReader := mocksnetpoll.NewMockReader(ctrl)
	mockReader.EXPECT().Next(validReadLength).Return([]byte(expectedString), nil).Times(2) // readBinary also calls reader.Next
	mockReader.EXPECT().Next(eofReadLength).Return(nil, io.EOF).Times(1)
	mockReader.EXPECT().Skip(validSkipLength).Return(nil).Times(1)
	mockReader.EXPECT().Skip(eofSkipLength).Return(io.EOF).Times(1)
	mockReader.EXPECT().Release().Return(nil).Times(1)
	reader := &netpollBufioxReader{
		Reader: mockReader,
	}
	currReadLen := 0

	// Next
	p, err := reader.Next(validReadLength)
	test.Assert(t, err == nil)
	test.Assert(t, "Hello" == string(p))
	test.Assert(t, validReadLength == reader.ReadLen())
	currReadLen = validReadLength
	p, err = reader.Next(8)
	test.Assert(t, io.EOF == err)
	test.Assert(t, p == nil)

	// ReadBinary
	buf := make([]byte, validReadLength)
	n, err := reader.ReadBinary(buf)
	test.Assert(t, err == nil)
	test.Assert(t, validReadLength == n)
	test.Assert(t, "Hello" == string(buf))
	test.Assert(t, currReadLen+validReadLength == reader.ReadLen())
	currReadLen = reader.ReadLen()

	// Skip
	err = reader.Skip(validReadLength)
	test.Assert(t, err == nil)
	test.Assert(t, currReadLen+validReadLength == reader.ReadLen())
	err = reader.Skip(3)
	test.Assert(t, io.EOF == err)

	// Release
	err = reader.Release(nil)
	test.Assert(t, err == nil)
	test.Assert(t, reader.ReadLen() == 0)
}
