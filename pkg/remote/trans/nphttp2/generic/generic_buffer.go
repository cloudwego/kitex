package generic

import (
	"bytes"
	"net"
	"sync"

	"github.com/cloudwego/kitex/pkg/remote"
)

// GenericConn implement WriteFrame and ReadFrame
type GenericConn interface {
	net.Conn
	// WriteFrame set header and data buffer into frame with nocopy
	WriteFrame(hdr, data []byte) (n int, err error)
	// ReadFrame read a frame and return header and payload data
	ReadFrame() (hdr, data []byte, err error)
}

type Buffer struct {
	conn       GenericConn
	whdr, wbuf []byte        // for write
	wSubBuf    *bytes.Buffer // for write // TODO: name
}

var (
	_ remote.ByteBuffer = (*Buffer)(nil)
	_ remote.FrameWrite = (*Buffer)(nil)
)

var gBufferPool = sync.Pool{
	New: func() interface{} {
		return &Buffer{}
	},
}

func NewGenericBuffer(conn GenericConn) *Buffer {
	buf := gBufferPool.Get().(*Buffer)
	buf.conn = conn
	buf.wSubBuf = &bytes.Buffer{}
	return buf
}

func (b *Buffer) WriteHeader(buf []byte) (err error) {
	b.whdr = buf
	return nil
}

func (b *Buffer) WriteData(buf []byte) (err error) {
	_, err = b.wSubBuf.Write(buf)
	return err
}

func (b *Buffer) Flush() (err error) {
	_, err = b.conn.WriteFrame(b.whdr, b.wbuf)
	b.whdr = nil
	b.wbuf = nil
	b.wSubBuf.Reset()
	return err
}

func (b *Buffer) Release(e error) (err error) {
	b.conn = nil
	b.whdr = nil
	b.wbuf = nil
	b.wSubBuf.Reset()
	gBufferPool.Put(b)
	return e
}

func (b *Buffer) GetData() []byte {
	return b.wSubBuf.Bytes()
}

func (b *Buffer) WritePayload(data []byte) (err error) {
	b.wbuf = data
	return nil
}

// === unimplemented ===

func (b *Buffer) Next(n int) (p []byte, err error) {
	panic("implement me")
	//b.growRbuf(n)
	//_, err = b.conn.Read(b.rbuf[:n])
	//if err != nil {
	//	return nil, err
	//}
	//return b.rbuf[:n], nil
}

func (b *Buffer) Read(p []byte) (n int, err error) {
	panic("implement me")
}

func (b *Buffer) Write(p []byte) (n int, err error) {
	panic("implement me")
}

func (b *Buffer) Peek(n int) (buf []byte, err error) {
	panic("implement me")
}

func (b *Buffer) Skip(n int) (err error) {
	panic("implement me")
}

func (b *Buffer) ReadableLen() (n int) {
	panic("implement me")
}

func (b *Buffer) ReadLen() (n int) {
	panic("implement me")
}

func (b *Buffer) ReadString(n int) (s string, err error) {
	panic("implement me")
}

func (b *Buffer) ReadBinary(n int) (p []byte, err error) {
	panic("implement me")
}

func (b *Buffer) Malloc(n int) (buf []byte, err error) {
	panic("implement me")
}

func (b *Buffer) MallocLen() (length int) {
	panic("implement me")
}

func (b *Buffer) WriteString(s string) (n int, err error) {
	panic("implement me")
}

func (b *Buffer) WriteBinary(data []byte) (n int, err error) {
	panic("implement me")
}

func (b *Buffer) NewBuffer() remote.ByteBuffer {
	panic("implement me")
}

func (b *Buffer) AppendBuffer(buf remote.ByteBuffer) (err error) {
	panic("implement me")
}

func (b *Buffer) Bytes() (buf []byte, err error) {
	panic("implement me")
}
