package allocator

import (
	"reflect"
	"unsafe"
)

func New[T any](ac Allocator, size ...int) (ptr *T) {
	if ac == nil {
		return new(T)
	}
	var s int
	if len(size) > 0 {
		s = size[0]
	} else {
		var t T
		s = int(unsafe.Sizeof(t))
	}
	ptr = (*T)(ac.Alloc(s))
	return ptr
}

func NewSlice[T any](ac Allocator, size int) (s []T) {
	if ac == nil {
		return make([]T, size)
	}
	var t *T
	hasptr := hasPtr(reflect.TypeOf(t).Elem().Kind())
	if hasptr {
		// if slice element type is pointer, we should create native object
		s = make([]T, size, size)
		ac.Refer(s)
	} else {
		data := ac.Alloc(int(unsafe.Sizeof(*t)) * size)
		shdr := &sliceHeader{Data: data, Len: int64(size), Cap: int64(size)}
		s = *(*[]T)(unsafe.Pointer(shdr))
	}
	return s
}

func NewMap[K comparable, V any](ac Allocator, size int) map[K]V {
	m := make(map[K]V, size)
	if ac != nil {
		ac.Refer(m)
	}
	return m
}

func hasPtr(k reflect.Kind) bool {
	switch k {
	case reflect.Bool,
		reflect.Int,
		reflect.Int8,
		reflect.Int16,
		reflect.Int32,
		reflect.Int64,
		reflect.Uint,
		reflect.Uint8,
		reflect.Uint16,
		reflect.Uint32,
		reflect.Uint64,
		reflect.Float32,
		reflect.Float64,
		reflect.Complex64,
		reflect.Complex128:
		return false
	}
	return true
}
