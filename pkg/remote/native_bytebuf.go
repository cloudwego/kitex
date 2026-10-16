package remote

import (
	"errors"
	"io"
)

type nativeByteBuffer struct {
	data          []byte
	read, written int
	writer        bool
}

func NewNativeWriter(b NativeBuffer) ByteBuffer {
	return &nativeByteBuffer{data: b.Bytes(), writer: true}
}

func NewNativeReader(b NativeBuffer) ByteBuffer {
	return &nativeByteBuffer{data: b.Bytes()}
}

func (b *nativeByteBuffer) Next(n int) ([]byte, error) {
	if b.writer || n < 0 || n > len(b.data)-b.read {
		return nil, io.ErrUnexpectedEOF
	}
	p := b.data[b.read : b.read+n]
	b.read += n
	return p, nil
}
func (b *nativeByteBuffer) Peek(n int) ([]byte, error) {
	if b.writer || n < 0 || n > len(b.data)-b.read {
		return nil, io.ErrUnexpectedEOF
	}
	return b.data[b.read : b.read+n], nil
}
func (b *nativeByteBuffer) Skip(n int) error    { _, err := b.Next(n); return err }
func (b *nativeByteBuffer) Release(error) error { b.data = nil; b.read = 0; b.written = 0; return nil }
func (b *nativeByteBuffer) ReadableLen() int {
	if b.writer {
		return -1
	}
	return len(b.data) - b.read
}
func (b *nativeByteBuffer) ReadLen() int { return b.read }
func (b *nativeByteBuffer) ReadString(n int) (string, error) {
	p, err := b.Next(n)
	return string(p), err
}
func (b *nativeByteBuffer) ReadBinary(p []byte) (int, error) { return b.Read(p) }
func (b *nativeByteBuffer) Read(p []byte) (int, error) {
	if b.writer {
		return 0, errors.New("native buffer is write-only")
	}
	if len(p) == 0 {
		return 0, nil
	}
	if b.read == len(b.data) {
		return 0, io.EOF
	}
	n := copy(p, b.data[b.read:])
	b.read += n
	return n, nil
}
func (b *nativeByteBuffer) Malloc(n int) ([]byte, error) {
	if !b.writer || n < 0 || n > len(b.data)-b.written {
		return nil, io.ErrShortBuffer
	}
	p := b.data[b.written : b.written+n]
	b.written += n
	return p, nil
}
func (b *nativeByteBuffer) WrittenLen() int             { return b.written }
func (b *nativeByteBuffer) Write(p []byte) (int, error) { return b.WriteBinary(p) }
func (b *nativeByteBuffer) WriteString(s string) (int, error) {
	p, err := b.Malloc(len(s))
	if err != nil {
		return 0, err
	}
	return copy(p, s), nil
}
func (b *nativeByteBuffer) WriteBinary(p []byte) (int, error) {
	out, err := b.Malloc(len(p))
	if err != nil {
		return 0, err
	}
	return copy(out, p), nil
}
func (b *nativeByteBuffer) Flush() error          { return nil }
func (b *nativeByteBuffer) NewBuffer() ByteBuffer { return NewWriterBuffer(256) }
func (b *nativeByteBuffer) AppendBuffer(other ByteBuffer) error {
	p, err := other.Bytes()
	if err != nil {
		return err
	}
	_, err = b.WriteBinary(p)
	return err
}
func (b *nativeByteBuffer) Bytes() ([]byte, error) {
	if b.writer {
		return b.data[:b.written], nil
	}
	return b.data[b.read:], nil
}
