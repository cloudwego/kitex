package allocator

import "unsafe"

//go:linkname memclrNoHeapPointers reflect.memclrNoHeapPointers
//go:noescape
func memclrNoHeapPointers(ptr unsafe.Pointer, n uintptr)

//go:linkname memmoveNoHeapPointers reflect.memmove
//go:noescape
func memmoveNoHeapPointers(to, from unsafe.Pointer, n uintptr)

type iface struct {
	tab  unsafe.Pointer
	data unsafe.Pointer
}

type sliceHeader struct {
	Data unsafe.Pointer // 8
	Len  int64          // 8
	Cap  int64          // 8
}

type stringHeader struct {
	Data unsafe.Pointer
	Len  int
}

func aligned(n int) int {
	if n%ptrSize != 0 {
		n = (n + ptrSize + 1) & ^(ptrSize - 1)
	}
	return n
}
