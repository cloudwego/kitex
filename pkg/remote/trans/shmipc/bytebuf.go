package shmipc

import (
	"errors"
	"sync"

	"github.com/cloudwego/shmipc-go"

	"github.com/cloudwego/kitex/pkg/remote"
)

var (
	bytebufPool sync.Pool
	_           remote.ByteBuffer = &shmIPCByteBuffer{}
)

func init() {
	bytebufPool.New = newShmIPCByteBuffer
}

func newShmIPCByteBuffer() interface{} {
	return &shmIPCByteBuffer{}
}

func newReaderByteBuffer(s *shmipc.Stream) remote.ByteBuffer {
	bytebuf := bytebufPool.Get().(*shmIPCByteBuffer)
	bytebuf.stream = s
	bytebuf.status = remote.BitReadable
	bytebuf.readIdx = 0
	return bytebuf
}

func newWriterByteBuffer(s *shmipc.Stream) remote.ByteBuffer {
	bytebuf := bytebufPool.Get().(*shmIPCByteBuffer)
	bytebuf.stream = s
	bytebuf.status = remote.BitWritable
	bytebuf.writeIdx = 0
	return bytebuf
}

type shmIPCByteBuffer struct {
	stream   *shmipc.Stream
	status   int
	readIdx  int
	writeIdx int
}

func (b *shmIPCByteBuffer) Read(p []byte) (n int, err error) {
	return b.stream.Read(p)
}

func (b *shmIPCByteBuffer) Write(p []byte) (n int, err error) {
	return b.stream.Write(p)
}

// cannot be called concurrent
func (b *shmIPCByteBuffer) Next(n int) (buf []byte, err error) {
	buf, err = b.stream.BufferReader().ReadBytes(n)
	b.readIdx += n
	return buf, err
}

func (b *shmIPCByteBuffer) Peek(n int) (buf []byte, err error) {
	return b.stream.BufferReader().Peek(n)
}

func (b *shmIPCByteBuffer) Skip(n int) (err error) {
	n, err = b.stream.BufferReader().Discard(n)
	b.readIdx += n
	return err
}

func (b *shmIPCByteBuffer) Release(e error) (err error) {
	if b.status&remote.BitReadable == 1 && b.stream != nil {
		b.stream.ReleaseReadAndReuse()
	}
	b.zero()
	bytebufPool.Put(b)
	return
}

func (b *shmIPCByteBuffer) ReadableLen() (n int) {
	return b.stream.BufferReader().Len()
}

func (b *shmIPCByteBuffer) ReadLen() (n int) {
	return b.readIdx
}

func (b *shmIPCByteBuffer) ReadString(n int) (s string, err error) {
	if n < 0 {
		return "", nil
	}
	return b.stream.BufferReader().ReadString(n)
}

func (b *shmIPCByteBuffer) ReadBinary(p []byte) (n int, err error) {
	if n < 0 {
		return 0, errors.New("invalid binary length")
	}
	n = len(p)
	src, err := b.Next(n)
	if err != nil {
		return 0, err
	}
	copy(p, src)
	return n, nil
}

func (b *shmIPCByteBuffer) Malloc(n int) (buf []byte, err error) {
	buf, err = b.stream.BufferWriter().Reserve(n)
	b.writeIdx += n
	return buf, err
}

// 当前已经开辟的 buffer 总大小
func (b *shmIPCByteBuffer) WrittenLen() (length int) {
	return b.writeIdx
}

func (b *shmIPCByteBuffer) WriteString(s string) (n int, err error) {
	if err = b.stream.BufferWriter().WriteString(s); err != nil {
		return 0, err
	}
	n = len(s)
	b.writeIdx += n
	return n, nil
}

func (b *shmIPCByteBuffer) WriteBinary(bs []byte) (n int, err error) {
	b.writeIdx += len(bs)
	return b.stream.BufferWriter().WriteBytes(bs)
}

func (b *shmIPCByteBuffer) Flush() (err error) {
	return b.stream.Flush(false)
}

// NewBuffer not support in shmipc
func (b *shmIPCByteBuffer) NewBuffer() remote.ByteBuffer {
	panic("NewBuffer() not support in shmipc bytebuf")
}

// AppendBuffer not support in shmipc
func (b *shmIPCByteBuffer) AppendBuffer(buf remote.ByteBuffer) (err error) {
	return errors.New("AppendBuffer() not support in shmipc bytebuf")
}

// Bytes return the backing byte array of this buffer
func (b *shmIPCByteBuffer) Bytes() (buf []byte, err error) {
	return nil, errors.New("Bytes() not support in shmIPC bytebuf")
}

func (b *shmIPCByteBuffer) zero() {
	b.status = 0
	b.stream = nil
	b.readIdx = 0
	b.writeIdx = 0
}
