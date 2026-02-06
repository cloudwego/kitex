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
	"context"
	"io"
	"testing"
	"time"

	"github.com/cloudwego/kitex/internal/test"
)

func TestEncoding(t *testing.T) {
	testBytes := []byte("testbytes")
	encodingStr := encodeBinHeader(testBytes)
	decodingBytes, err := decodeBinHeader(encodingStr)
	test.Assert(t, err == nil, err)
	test.Assert(t, string(decodingBytes) == string(testBytes))

	testKey, testValue := "key-bin", "value"
	encodeValue := encodeMetadataHeader(testKey, testValue)
	decodeValue, err := decodeMetadataHeader(testKey, encodeValue)
	test.Assert(t, err == nil, err)
	test.Assert(t, decodeValue == testValue)

	testEncoding := "%testcoding"
	encodedTestStr := encodeGrpcMessage(testEncoding)
	decodedTestStr := decodeGrpcMessage(encodedTestStr)
	test.Assert(t, decodedTestStr == testEncoding)
}

func TestEncodeTimeout(t *testing.T) {
	_, err := decodeTimeout("")
	test.Assert(t, err != nil, err)

	_, err = decodeTimeout("#####")
	test.Assert(t, err != nil, err)

	_, err = decodeTimeout("1234567890123")
	test.Assert(t, err != nil, err)

	to := encodeTimeout(-1)
	test.Assert(t, to == "0n")
	d, err := decodeTimeout(to)
	test.Assert(t, err == nil, err)
	test.Assert(t, d == 0)

	to = encodeTimeout(time.Nanosecond)
	test.Assert(t, to == "1n")
	d, err = decodeTimeout(to)
	test.Assert(t, err == nil, err)
	test.Assert(t, d == time.Nanosecond)

	to = encodeTimeout(time.Microsecond)
	test.Assert(t, to == "1000n")
	d, err = decodeTimeout(to)
	test.Assert(t, err == nil, err)
	test.Assert(t, d == time.Microsecond)

	to = encodeTimeout(time.Millisecond)
	test.Assert(t, to == "1000000n")
	d, err = decodeTimeout(to)
	test.Assert(t, err == nil, err)
	test.Assert(t, d == time.Millisecond)

	to = encodeTimeout(time.Second)
	test.Assert(t, to == "1000000u")
	d, err = decodeTimeout(to)
	test.Assert(t, err == nil, err)
	test.Assert(t, d == time.Second)

	to = encodeTimeout(time.Minute)
	test.Assert(t, to == "60000000u")
	d, err = decodeTimeout(to)
	test.Assert(t, err == nil, err)
	test.Assert(t, d == time.Minute)

	to = encodeTimeout(time.Hour)
	test.Assert(t, to == "3600000m")
	d, err = decodeTimeout(to)
	test.Assert(t, err == nil, err)
	test.Assert(t, d == time.Hour)

	to = encodeTimeout(time.Minute * 1000000)
	test.Assert(t, to == "60000000S")
	d, err = decodeTimeout(to)
	test.Assert(t, err == nil, err)
	test.Assert(t, d == time.Minute*1000000)

	to = encodeTimeout(time.Hour * 1000000)
	test.Assert(t, to == "60000000M")
	d, err = decodeTimeout(to)
	test.Assert(t, err == nil, err)
	test.Assert(t, d == time.Hour*1000000)

	to = encodeTimeout(time.Hour * 2000000)
	test.Assert(t, to == "2000000H")
	d, err = decodeTimeout(to)
	test.Assert(t, err == nil, err)
	test.Assert(t, d == time.Hour*2000000)
}

func TestTimeUnit(t *testing.T) {
	d, ok := timeoutUnitToDuration(timeoutUnit('H'))
	test.Assert(t, ok)
	test.Assert(t, d == time.Hour)

	d, ok = timeoutUnitToDuration(timeoutUnit('M'))
	test.Assert(t, ok)
	test.Assert(t, d == time.Minute)

	d, ok = timeoutUnitToDuration(timeoutUnit('S'))
	test.Assert(t, ok)
	test.Assert(t, d == time.Second)

	d, ok = timeoutUnitToDuration(timeoutUnit('m'))
	test.Assert(t, ok)
	test.Assert(t, d == time.Millisecond)

	d, ok = timeoutUnitToDuration(timeoutUnit('u'))
	test.Assert(t, ok)
	test.Assert(t, d == time.Microsecond)

	d, ok = timeoutUnitToDuration(timeoutUnit('n'))
	test.Assert(t, ok)
	test.Assert(t, d == time.Nanosecond)

	_, ok = timeoutUnitToDuration(timeoutUnit('x'))
	test.Assert(t, !ok)
}

func TestBufferWriter(t *testing.T) {
	t.Run("keep", func(t *testing.T) {
		bytes := []byte("hello")
		br := newBufWriter(newMockNpConn(mockAddr0), len(bytes)*5, ReuseWriteBufferConfig{})
		for i := 0; i < 6; i++ {
			_, err := br.Write(bytes)
			test.Assert(t, err == nil, err)
		}
		err := br.Flush()
		test.Assert(t, err == nil, err)
	})
	t.Run("keep-flush-error-on-large-write", func(t *testing.T) {
		writeErr := io.ErrShortWrite
		var writeCount int
		mockWriter := &mockConn{
			WriteFunc: func(b []byte) (int, error) {
				writeCount++
				return 0, writeErr
			},
		}

		batchSize := 50
		br := newBufWriter(mockWriter, batchSize, ReuseWriteBufferConfig{Enable: false}).(*keepBufWriter)

		// Write large data that would trigger multiple flushes if not stopped
		largeData := make([]byte, 500)
		for i := range largeData {
			largeData[i] = byte(i % 256)
		}

		n, err := br.Write(largeData)
		test.Assert(t, err == writeErr, err)
		test.Assert(t, n == 100, n)
		test.Assert(t, writeCount == 1, writeCount)
		test.Assert(t, br.err == writeErr, br.err)

		n, err = br.Write([]byte("more"))
		test.Assert(t, err == writeErr, err)
		test.Assert(t, n == 0, n)
		test.Assert(t, writeCount == 1, writeCount)
	})
	t.Run("reuse", func(t *testing.T) {
		t.Run("normal", func(t *testing.T) {
			var writtenData []byte
			mockWriter := &mockConn{
				WriteFunc: func(b []byte) (int, error) {
					writtenData = append(writtenData, b...)
					return len(b), nil
				},
			}

			batchSize := 100
			br := newBufWriter(mockWriter, batchSize, ReuseWriteBufferConfig{Enable: true}).(*reuseBufWriter)

			data1 := []byte("test data")
			n, err := br.Write(data1)
			test.Assert(t, err == nil, err)
			test.Assert(t, n == len(data1), n)
			test.Assert(t, br.buf != nil)
			test.Assert(t, len(writtenData) == 0, len(writtenData))

			err = br.Flush()
			test.Assert(t, err == nil, err)
			test.Assert(t, string(writtenData) == string(data1), string(writtenData))
			test.Assert(t, br.buf == nil, br.buf)

			writtenData = nil
			data2 := []byte("new data")
			n, err = br.Write(data2)
			test.Assert(t, err == nil, err)
			test.Assert(t, n == len(data2), n)
			test.Assert(t, br.buf != nil)

			err = br.Flush()
			test.Assert(t, err == nil, err)
			test.Assert(t, string(writtenData) == string(data2), string(writtenData))
			test.Assert(t, br.buf == nil, br.buf)
		})
		t.Run("auto-flush", func(t *testing.T) {
			var flushCount int
			var totalWritten int
			mockWriter := &mockConn{
				WriteFunc: func(b []byte) (int, error) {
					flushCount++
					totalWritten += len(b)
					return len(b), nil
				},
			}

			batchSize := 50
			br := newBufWriter(mockWriter, batchSize, ReuseWriteBufferConfig{Enable: true}).(*reuseBufWriter)

			data := make([]byte, batchSize)
			for i := range data {
				data[i] = byte(i % 256)
			}

			n, err := br.Write(data)
			test.Assert(t, err == nil, err)
			test.Assert(t, n == batchSize)
			test.Assert(t, flushCount == 1, flushCount)
			test.Assert(t, totalWritten == batchSize, totalWritten)
			test.Assert(t, br.GetOffset() == 0, br.GetOffset())

			data2 := make([]byte, 60)
			n, err = br.Write(data2)
			test.Assert(t, err == nil, err)
			test.Assert(t, n == len(data2))
			test.Assert(t, flushCount == 2, flushCount)
			test.Assert(t, br.GetOffset() == 0, br.GetOffset())

			data3 := make([]byte, 30)
			n, err = br.Write(data3)
			test.Assert(t, err == nil, err)
			test.Assert(t, n == len(data3), n)
			test.Assert(t, flushCount == 2, flushCount)
			test.Assert(t, br.GetOffset() == 30, br.GetOffset())

			err = br.Flush()
			test.Assert(t, err == nil, err)
			test.Assert(t, flushCount == 3, flushCount)
			test.Assert(t, totalWritten == batchSize+60+30, totalWritten)
		})
		t.Run("large-write", func(t *testing.T) {
			var chunks [][]byte
			mockWriter := &mockConn{
				WriteFunc: func(b []byte) (int, error) {
					chunk := make([]byte, len(b))
					copy(chunk, b)
					chunks = append(chunks, chunk)
					return len(b), nil
				},
			}

			batchSize := 100
			br := newBufWriter(mockWriter, batchSize, ReuseWriteBufferConfig{Enable: true}).(*reuseBufWriter)

			largeData := make([]byte, 250)
			for i := range largeData {
				largeData[i] = byte(i % 256)
			}

			n, err := br.Write(largeData)
			test.Assert(t, err == nil, err)
			test.Assert(t, n == len(largeData), n)

			test.Assert(t, len(chunks) == 1, len(chunks))
			test.Assert(t, len(chunks[0]) == 200, len(chunks[0]))
			test.Assert(t, br.GetOffset() == 50, br.GetOffset())

			err = br.Flush()
			test.Assert(t, err == nil, err)
			test.Assert(t, len(chunks) == 2, len(chunks))

			var reconstructed []byte
			for _, chunk := range chunks {
				reconstructed = append(reconstructed, chunk...)
			}
			test.Assert(t, len(reconstructed) == len(largeData))
			for i := range largeData {
				test.Assert(t, reconstructed[i] == largeData[i], reconstructed, largeData)
			}
		})
		t.Run("empty-write", func(t *testing.T) {
			mockWriter := &mockConn{
				WriteFunc: func(b []byte) (int, error) {
					return len(b), nil
				},
			}

			br := newBufWriter(mockWriter, 100, ReuseWriteBufferConfig{Enable: true}).(*reuseBufWriter)

			n, err := br.Write([]byte{})
			test.Assert(t, err == nil, err)
			test.Assert(t, n == 0, n)
			test.Assert(t, br.GetOffset() == 0, br.GetOffset())

			err = br.Flush()
			test.Assert(t, err == nil, err)

			for i := 0; i < 5; i++ {
				n, err = br.Write(nil)
				test.Assert(t, err == nil, err)
				test.Assert(t, n == 0, n)
			}
		})
		t.Run("disabled-buffering", func(t *testing.T) {
			var writeCount int
			mockWriter := &mockConn{
				WriteFunc: func(b []byte) (int, error) {
					writeCount++
					return len(b), nil
				},
			}

			br := newBufWriter(mockWriter, 0, ReuseWriteBufferConfig{Enable: true})

			data := []byte("test")
			n, err := br.Write(data)
			test.Assert(t, err == nil, err)
			test.Assert(t, n == len(data), n)
			test.Assert(t, writeCount == 1, writeCount)

			n, err = br.Write(data)
			test.Assert(t, err == nil, err)
			test.Assert(t, n == len(data), n)
			test.Assert(t, writeCount == 2, writeCount)
		})
		t.Run("write-error", func(t *testing.T) {
			writeErr := io.ErrShortWrite
			mockWriter := &mockConn{
				WriteFunc: func(b []byte) (int, error) {
					return 0, writeErr
				},
			}

			batchSize := 50
			br := newBufWriter(mockWriter, batchSize, ReuseWriteBufferConfig{Enable: true}).(*reuseBufWriter)

			data := make([]byte, batchSize)
			_, err := br.Write(data)
			test.Assert(t, err == writeErr, err)
			test.Assert(t, br.err == writeErr, br.err)

			_, err = br.Write([]byte("more"))
			test.Assert(t, err == writeErr, err)

			err = br.Flush()
			test.Assert(t, err == writeErr, err)
		})
		t.Run("flush-error-on-large-write", func(t *testing.T) {
			writeErr := io.ErrShortWrite
			var writeCount int
			mockWriter := &mockConn{
				WriteFunc: func(b []byte) (int, error) {
					writeCount++
					return 0, writeErr
				},
			}

			batchSize := 50
			br := newBufWriter(mockWriter, batchSize, ReuseWriteBufferConfig{Enable: true}).(*reuseBufWriter)

			largeData := make([]byte, 500)
			for i := range largeData {
				largeData[i] = byte(i % 256)
			}

			n, err := br.Write(largeData)
			test.Assert(t, err == writeErr, err)
			test.Assert(t, n == 100, n)
			test.Assert(t, writeCount == 1, writeCount)
			test.Assert(t, br.err == writeErr, br.err)
			test.Assert(t, br.buf != nil, br.buf)

			n, err = br.Write([]byte("more"))
			test.Assert(t, err == writeErr, err)
			test.Assert(t, n == 0, n)
			test.Assert(t, writeCount == 1, writeCount)

			err = br.Flush()
			test.Assert(t, err == writeErr, err)
			test.Assert(t, br.buf != nil, "buffer should not be freed on flush error")
		})
		t.Run("multiple-flushes", func(t *testing.T) {
			var flushCount int
			mockWriter := &mockConn{
				WriteFunc: func(b []byte) (int, error) {
					if len(b) > 0 {
						flushCount++
					}
					return len(b), nil
				},
			}

			br := newBufWriter(mockWriter, 100, ReuseWriteBufferConfig{Enable: true})

			err := br.Flush()
			test.Assert(t, err == nil, err)
			test.Assert(t, flushCount == 0, flushCount)

			br.Write([]byte("data"))
			err = br.Flush()
			test.Assert(t, err == nil, err)
			test.Assert(t, flushCount == 1, flushCount)

			err = br.Flush()
			test.Assert(t, err == nil, err)
			test.Assert(t, flushCount == 1, flushCount)

			br.Write([]byte("more"))
			br.Flush()
			br.Write([]byte("data"))
			br.Flush()
			test.Assert(t, flushCount == 3)
		})
	})
}

func TestBufferPool(t *testing.T) {
	pool := newBufferPool()
	buffer := pool.get()
	pool.put(buffer)
}

func TestRecvBufferReader(t *testing.T) {
	testBytes := []byte("hello")
	buffer := new(bytes.Buffer)
	buffer.Reset()
	buffer.Write(testBytes)
	recvBuffer := newRecvBuffer()
	recvBuffer.put(recvMsg{buffer: buffer})
	ctx := context.Background()
	rbReader := &recvBufferReader{
		ctx:     ctx,
		ctxDone: ctx.Done(),
		recv:    recvBuffer,
		closeStream: func(err error) {
		},
		freeBuffer: func(buffer *bytes.Buffer) {
			buffer.Reset()
		},
	}

	n, err := rbReader.Read(testBytes)
	test.Assert(t, err == nil, err)
	test.Assert(t, n == len(testBytes))

	buffer.Write(testBytes)
	recvBuffer.put(recvMsg{buffer: buffer})
	rbReader.closeStream = nil
	n, err = rbReader.Read(testBytes)
	test.Assert(t, err == nil, err)
	test.Assert(t, n == len(testBytes))
}

func TestConnectionError(t *testing.T) {
	connectionError := ConnectionError{
		Desc: "err desc",
		temp: true,
		err:  nil,
	}

	errStr := connectionError.Error()
	test.Assert(t, errStr == "connection error: desc = \"err desc\"")

	temp := connectionError.Temporary()
	test.Assert(t, temp)

	ori := connectionError.Origin()
	test.Assert(t, ori == connectionError)

	connectionError.err = context.Canceled
	ori = connectionError.Origin()
	test.Assert(t, ori == context.Canceled)

	test.Assert(t, connectionError.Code() == 14)
}
