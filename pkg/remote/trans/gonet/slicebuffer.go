package gonet

import (
	"fmt"
	"io"
	"sync"

	"github.com/cloudwego/kitex/pkg/utils"

	"github.com/bytedance/gopkg/lang/dirtmake"
	"github.com/bytedance/gopkg/lang/mcache"
)

const defaultChunkSize = 4096

var sliceBufferPool = sync.Pool{
	New: func() interface{} {
		return newSliceBuffer(defaultChunkSize)
	},
}

var readerPool = sync.Pool{
	New: func() interface{} {
		return &reader{}
	},
}

var writerPool = sync.Pool{
	New: func() interface{} {
		return &writer{}
	},
}

// sliceBuffer 负责内存管理
type sliceBuffer struct {
	chunks    [][]byte
	chunkSize int

	// write
	writeChunk  int
	writeOffset int
	writeSize   int
	// read
	readChunk  int
	readOffset int
	readSize   int
}

func newSliceBuffer(chunkSize int) *sliceBuffer {
	var dummy []byte
	sb := &sliceBuffer{
		chunks:    [][]byte{dummy},
		chunkSize: chunkSize,
		readChunk: 1,
	}
	return sb
}

// malloc 分配 `size` 大小的 `[]byte`
func (b *sliceBuffer) malloc(size int) []byte {
	if len(b.chunks[b.writeChunk])-b.writeOffset < size {
		// new chunk
		var newChunk []byte
		if size > b.chunkSize {
			newChunk = mcache.Malloc(size, size) // make([]byte, size)
		} else {
			newChunk = mcache.Malloc(b.chunkSize, b.chunkSize) // make([]byte, b.chunkSize)
		}
		b.chunks[b.writeChunk] = b.chunks[b.writeChunk][:b.writeOffset] // fix the curr chunk
		b.chunks = append(b.chunks, newChunk)                           // append the new chunk
		b.writeChunk++
		b.writeOffset = 0
	}

	buf := b.chunks[b.writeChunk][b.writeOffset : b.writeOffset+size]
	b.writeOffset += size
	b.writeSize += size

	return buf
}

func (b *sliceBuffer) read(size int, move, skip bool) ([]byte, error) {
	if size == 0 {
		return nil, nil
	}

	if b.readableLen() < size || len(b.chunks) <= b.readChunk {
		return nil, fmt.Errorf("not enough data to read")
	}

	ck := b.chunks[b.readChunk]
	start, end := b.readOffset, b.readOffset+size
	if end <= len(ck) {
		if move {
			b.readOffset = end
			b.readSize += size
		}
		if end == len(ck) {
			// to next chunk
			b.readChunk++
			b.readOffset = 0
		}
		// if not crossing chunk, no need to copy
		return ck[start:end], nil
	}

	var (
		result         []byte
		readBytes      = 0
		currChunk      = b.readChunk
		currReadOffset = b.readOffset
	)
	if !skip {
		result = dirtmake.Bytes(size, size)
	}
	for readBytes < size && currChunk < len(b.chunks) {
		ck = b.chunks[currChunk]
		start, end = currReadOffset, currReadOffset+(size-readBytes)
		if end >= len(ck) {
			// do not cross the chunk
			// if the current chunk is finished, move to the next chunk
			end = len(ck)
			currChunk++
			currReadOffset = 0
		} else {
			currReadOffset += end - start
		}
		if !skip {
			copy(result[readBytes:], ck[start:end])
		}
		readBytes += end - start
	}
	if move {
		b.readOffset = currReadOffset
		b.readChunk = currChunk
		b.readSize += size
	}

	return result, nil
}

func (b *sliceBuffer) next(size int) ([]byte, error) {
	return b.read(size, true, false)
}

func (b *sliceBuffer) peek(size int) ([]byte, error) {
	return b.read(size, false, false)
}

func (b *sliceBuffer) skip(size int) error {
	_, err := b.read(size, true, true)
	return err
}

// releaseUnused is only used and effective on the latest malloced buffer.
func (b *sliceBuffer) releaseUnused(n int) {
	if b.writeOffset-n >= 0 {
		b.writeOffset -= n
		b.writeSize -= n
	}
}

// readableLen returns the size of unread buffer
func (b *sliceBuffer) readableLen() int {
	return b.writeSize - b.readSize
}

func (b *sliceBuffer) release() {
	for i := 1; i < len(b.chunks); i++ {
		mcache.Free(b.chunks[i])
	}
	b.chunks = b.chunks[:1]
	b.readOffset = 0
	b.readChunk = 1
	b.readSize = 0
	b.writeChunk = 0
	b.writeOffset = 0
	b.writeSize = 0
	sliceBufferPool.Put(b)
}

type reader struct {
	src    io.Reader
	buffer *sliceBuffer
}

// NewReader 创建一个新的 reader
func newReader(src io.Reader) *reader {
	r := readerPool.Get().(*reader)
	r.src = src
	r.buffer = sliceBufferPool.Get().(*sliceBuffer)
	return r
}

// waitRead 阻塞直到 buffer 内可读取的数据长度 >= n
func (r *reader) waitRead(n int) error {
	for {
		readableLen := r.buffer.readableLen()
		if readableLen >= n {
			return nil
		}
		if err := r.fill(n - readableLen); err != nil {
			return err
		}
	}
}

// fill 读取数据到 buffer
func (r *reader) fill(n int) error {
	if n < r.buffer.chunkSize {
		n = r.buffer.chunkSize
	}
	buf := r.buffer.malloc(n)
	readLen, err := r.src.Read(buf)

	if readLen > 0 && n-readLen > 0 {
		r.buffer.releaseUnused(n - readLen)
	}

	return err
}

// Next 读取 n 个字节
func (r *reader) Next(n int) ([]byte, error) {
	if err := r.waitRead(n); err != nil {
		return nil, err
	}
	return r.buffer.next(n)
}

// Peek 预览 n 个字节，不推进 readIdx
func (r *reader) Peek(n int) ([]byte, error) {
	if err := r.waitRead(n); err != nil {
		return nil, err
	}
	return r.buffer.peek(n)
}

// Skip 跳过 n 个字节
func (r *reader) Skip(n int) error {
	if err := r.waitRead(n); err != nil {
		return err
	}
	return r.buffer.skip(n)
}

// Release 释放 buffer
func (r *reader) Release() error {
	r.buffer.release()
	r.src = nil
	r.buffer = nil
	readerPool.Put(r)
	return nil
}

// Len 返回可读数据的长度
func (r *reader) Len() int {
	return r.buffer.readableLen()
}

func (r *reader) ReadString(n int) (string, error) {
	b, err := r.Next(n)
	if err != nil {
		return "", err
	}
	return utils.SliceByteToString(b), nil
}

// newWriter 创建一个新的 reader
func newWriter(dst io.Writer) *writer {
	w := writerPool.Get().(*writer)
	w.dst = dst
	w.buffer = sliceBufferPool.Get().(*sliceBuffer)
	return w
}

type writer struct {
	dst    io.Writer
	buffer *sliceBuffer
}

func (w *writer) Malloc(n int) ([]byte, error) {
	buf := w.buffer.malloc(n)
	if buf == nil {
		return nil, fmt.Errorf("malloc failed")
	}
	return buf, nil
}

func (w *writer) MallocLen() int {
	return w.buffer.writeSize
}

func (w *writer) MallocAck(n int) error {
	if n > w.buffer.writeSize {
		return fmt.Errorf("ack size exceeds allocated size")
	}
	w.buffer.releaseUnused(w.buffer.writeSize - n)
	return nil
}

func (w *writer) Append(w2 *writer) error {
	panic("not implemented")
	//data, err := w2.buffer.read(w2.buffer.readableLen(), true)
	//if err != nil {
	//	return err
	//}
	//_, err = w.WriteBinary(data)
	//return err
}

func (w *writer) WriteString(s string) (int, error) {
	return w.WriteBinary(utils.StringToSliceByte(s))
}

func (w *writer) WriteBinary(b []byte) (int, error) {
	buf, err := w.Malloc(len(b))
	if err != nil {
		return 0, err
	}
	copy(buf, b)
	return len(b), nil
}

// FIXME: optimize with nocopy
func (w *writer) WriteDirect(p []byte, remainCap int) error {
	if len(p)+remainCap > w.buffer.chunkSize {
		return fmt.Errorf("not enough space in buffer")
	}
	_, err := w.WriteBinary(p)
	return err
}

func (w *writer) Flush() error {
	// 读取 buffer 中所有数据
	data, err := w.buffer.read(w.buffer.readableLen(), true, false)
	if err != nil {
		return err
	}

	// 将数据写入 `w`
	_, err = w.dst.Write(data)
	if err != nil {
		return err
	}
	w.buffer.release()
	w.dst = nil
	w.buffer = nil
	writerPool.Put(w)
	return nil
}
