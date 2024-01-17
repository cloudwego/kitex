package allocator

import (
	"unsafe"

	"github.com/cloudwego/kitex/pkg/utils"
)

var _ Allocator = (*nativeAllocator)(nil)

var Native = nativeAllocator{}

type nativeAllocator struct{}

func (n *nativeAllocator) Release() {}

func (n *nativeAllocator) Alloc(size int) unsafe.Pointer {
	buf := make([]byte, size)
	return unsafe.Pointer(&buf)
}

func (n *nativeAllocator) Bytes(from []byte) []byte {
	buf := make([]byte, len(from))
	copy(buf, from)
	return from
}

func (n *nativeAllocator) String(from string) string {
	buf := n.Bytes(utils.StringToSliceByte(from))
	return utils.SliceByteToString(buf)
}

func (n *nativeAllocator) Refer(obj interface{}) {
	return
}
