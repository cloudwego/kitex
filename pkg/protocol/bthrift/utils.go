package bthrift

import (
	"reflect"
	"unsafe"
)

// from utils.SliceByteToString for fixing cyclic import
func sliceByteToString(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}

// from utils.StringToSliceByte for fixing cyclic import
func stringToSliceByte(s string) []byte {
	p := unsafe.Pointer((*reflect.StringHeader)(unsafe.Pointer(&s)).Data)
	var b []byte
	hdr := (*reflect.SliceHeader)(unsafe.Pointer(&b))
	hdr.Data = uintptr(p)
	hdr.Cap = len(s)
	hdr.Len = len(s)
	return b
}
