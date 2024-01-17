package allocator

import (
	"math/bits"
	"sync"
)

const maxSize = 46

// index contains []byte which cap is 1<<index
var caches [maxSize]sync.Pool

// calculates which pool to get from
func calcIndex(size int) int {
	if size == 0 {
		return 0
	}
	if isPowerOfTwo(size) {
		return bsr(size)
	}
	return bsr(size) + 1
}

// Malloc supports one or two integer argument.
// The size specifies the length of the returned slice, which means len(ret) == size.
// A second integer argument may be provided to specify the minimum capacity, which means cap(ret) >= cap.
func Malloc(size int) []byte {
	idx := calcIndex(size)
	var buf = caches[idx].Get()
	if buf != nil {
		//println("cached buffer", size)
		return (buf.([]byte))[:size]
	}
	capacity := 1 << idx
	//println("new buffer", size, capacity)
	return make([]byte, size, capacity)
}

// Free should be called when the buf is no longer used.
func Free(buf []byte) {
	size := cap(buf)
	if !isPowerOfTwo(size) {
		return
	}
	buf = buf[:0]
	caches[bsr(size)].Put(buf)
}

func bsr(x int) int {
	return bits.Len(uint(x)) - 1
}

func isPowerOfTwo(x int) bool {
	return (x & (-x)) == x
}
