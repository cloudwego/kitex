package allocator

import (
	"reflect"
	"runtime"
	"sync"
	"unsafe"
)

type Kind int

const (
	ptrSize = int(unsafe.Sizeof(uintptr(0)))

	Memory Kind = 1
	Map    Kind = 2
	Slice  Kind = 3
)

type Allocator interface {
	Alloc(size int) unsafe.Pointer
	Bytes(from []byte) []byte
	String(from string) string
	Refer(obj interface{})
	Release()
}

var _ Allocator = (*allocator)(nil)

type allocator struct {
	Data      unsafe.Pointer
	Len       int
	Cap       int
	finalizer func()
	// all fields bellow must have a longer lifetime than allocator data
	externalrefs *reflink // external pointers only could be released when all blocks is released by Go GC.
}

type Option func(ac *allocator)

func WithFinalizer(finalizer func()) Option {
	return func(ac *allocator) {
		ac.finalizer = finalizer
	}
}

var (
	aPool        = sync.Pool{New: func() interface{} { return new(allocator) }}
	linkNodePool = sync.Pool{New: func() interface{} { return new(refnode) }}
	linkListPool = sync.Pool{New: func() interface{} { return new(reflink) }}
)

func NewAllocator(capacity int, opts ...Option) *allocator {
	ac := aPool.Get().(*allocator)
	rn := linkNodePool.Get().(*refnode)
	ac.externalrefs = linkListPool.Get().(*reflink)
	ac.externalrefs.head = rn
	ac.externalrefs.tail = rn
	for _, opt := range opts {
		opt(ac)
	}
	ac.init(capacity)
	return ac
}

func (a *allocator) Release() {
	a.reset()
	aPool.Put(a)
}

func (a *allocator) reset() {
	a.Data = nil
	a.Len = 0
	a.Cap = 0
	a.finalizer = nil
	a.externalrefs = nil
}

func (a *allocator) init(capacity int) {
	data := Malloc(capacity)
	datahdr := *(*sliceHeader)(unsafe.Pointer(&data))
	datahdr.Len = 0
	size := int(datahdr.Len)
	capacity = int(datahdr.Cap)

	a.Data = datahdr.Data
	a.Len = size
	a.Cap = capacity

	// register block finalizer
	refs := a.externalrefs
	finalizer := a.finalizer
	runtime.SetFinalizer(&data[0], func(x *byte) {
		// reuse buffer
		shdr := &sliceHeader{Data: unsafe.Pointer(x), Len: int64(size), Cap: int64(capacity)}
		buf := *(*[]byte)(unsafe.Pointer(shdr))
		Free(buf)

		// release all reference
		ref := refs.head.next
		for ref != nil {
			nxt := ref.next
			ref.ptr = nil
			ref.next = nil
			linkNodePool.Put(ref)
			ref = nxt
		}
		refs.head = nil
		refs.tail = nil
		linkListPool.Put(refs)
		if finalizer != nil {
			finalizer()
		}
	})
}

func (a *allocator) Alloc(size int) unsafe.Pointer {
	ptr := a.alloc(size, true)
	//memclrNoHeapPointers(ptr, uintptr(alignedsize))
	return ptr
}

func (a *allocator) Bytes(from []byte) []byte {
	if len(from) == 0 {
		return from
	}
	fromhdr := *(*sliceHeader)(unsafe.Pointer(&from))
	ptr := a.alloc(int(fromhdr.Cap), false)
	memmoveNoHeapPointers(ptr, fromhdr.Data, uintptr(fromhdr.Len))
	shdr := sliceHeader{Data: ptr, Len: fromhdr.Len, Cap: fromhdr.Cap}
	return *(*[]byte)(unsafe.Pointer(&shdr))
}

func (a *allocator) String(from string) string {
	if len(from) == 0 {
		return from
	}
	fromhdr := *(*stringHeader)(unsafe.Pointer(&from))
	ptr := a.alloc(fromhdr.Len, false)
	memmoveNoHeapPointers(ptr, fromhdr.Data, uintptr(fromhdr.Len))
	shdr := stringHeader{Data: ptr, Len: fromhdr.Len}
	return *(*string)(unsafe.Pointer(&shdr))
}

func (a *allocator) Refer(obj interface{}) {
	ihdr := (*iface)(unsafe.Pointer(&obj))
	kind := reflect.TypeOf(obj).Kind()
	switch kind {
	case reflect.Slice:
		a.refer(ihdr.data, Slice)
	case reflect.Map:
		a.refer(ihdr.data, Map)
	}
}

func (a *allocator) refer(ptr unsafe.Pointer, kind Kind) {
	rn := linkNodePool.Get().(*refnode)
	rn.ptr = ptr
	rn.kind = kind
	a.externalrefs.tail.next = rn
	a.externalrefs.tail = a.externalrefs.tail.next
}

func (a *allocator) alloc(size int, align bool) (ptr unsafe.Pointer) {
	if align {
		a.Len = aligned(a.Len)
		size = aligned(size)
	}
	if size > (a.Cap - a.Len) {
		println("not enough", size, a.Cap, a.Len)
		return nil
	}
	ptr = unsafe.Add(a.Data, a.Len)
	a.Len += size
	return ptr
}

func (a *allocator) makeslice(typesize, size, capacity int) sliceHeader {
	spacesize := typesize * capacity
	ptr := a.alloc(spacesize, false)
	memclrNoHeapPointers(ptr, uintptr(spacesize))
	return sliceHeader{
		Data: ptr,
		Len:  int64(size),
		Cap:  int64(capacity),
	}
}

type reflink struct {
	head *refnode
	tail *refnode
}

type refnode struct {
	ptr  unsafe.Pointer
	kind Kind
	next *refnode
}
