package netpoll

import (
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/netpoll"
)

// NetpollWriter exposes the underlying writer for transport extensions.
// Buffer ownership and Release continue to belong to the original ByteBuffer.
func (b *netpollByteBuffer) NetpollWriter() netpoll.Writer {
	if b.status&remote.BitWritable == 0 {
		return nil
	}
	return b.writer
}
