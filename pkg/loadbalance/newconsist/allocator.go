package newconsist

import (
	"sync/atomic"
	"unsafe"
)

func NewAllocator(size int) *allocator {
	a := new(allocator)
	a.size = uint32(size)
	a.cache = make([]virtualNode, a.size)
	a.index2pointer = make(map[uint32]uintptr, a.size)
	a.kind = 1
	return a
}

type allocator struct {
	kind uint32 // 0=thread-unsafe, 1=thread-safe
	lock uint32
	read uint32 // read index of cache
	size uint32 // size of cache

	cache         []virtualNode
	index2pointer map[uint32]uintptr
}

func (a *allocator) tryLock() bool {
	return atomic.CompareAndSwapUint32(&a.lock, 0, 1)
}

func (a *allocator) unlock() {
	atomic.StoreUint32(&a.lock, 0)
}

func (a *allocator) Alloc() (t *virtualNode) {
	if a.kind > 0 && !a.tryLock() {
		return new(virtualNode)
	}
	var idx uint32
	if a.read < a.size {
		t = &a.cache[a.read]
		idx = a.read
		a.read++
	} else {
		a.cache = make([]virtualNode, a.size)
		a.read = 1
		t = &a.cache[0]
		idx = 0
	}
	if a.kind > 0 {
		a.unlock()
	}
	a.index2pointer[idx] = uintptr(unsafe.Pointer(t))
	return t
}

//
//func (a *allocator) Free() uint32 {
//	if a.kind > 0 {
//		defer a.unlock()
//		for {
//			if a.tryLock() {
//				break
//			} else {
//				runtime.Gosched()
//			}
//		}
//	}
//	currIdx := uint32(0)
//	lastIdx := (a.read) - 1
//	for currIdx < lastIdx {
//		if a.cache[currIdx].status != 0 {
//			// swap the last one
//			a.cache[currIdx] = a.cache[lastIdx]
//			lastIdx--
//		} else {
//			currIdx++
//		}
//	}
//	a.cache = append(a.cache[:lastIdx], a.cache[a.read:]...)
//	gap := a.read - lastIdx
//	a.read -= gap
//	a.size -= gap
//	return gap
//}
