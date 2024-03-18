package sys

import (
	"bytes"
	"encoding/binary"
	"testing"
	"unsafe"
)

func encodeList(list [][]byte) []byte {
	var buffer bytes.Buffer
	size := uint32(len(list))
	bytes4 := make([]byte, 4)
	binary.BigEndian.PutUint32(bytes4, size)
	buffer.Write(bytes4)
	for i := uint32(0); i < size; i++ {
		length := uint32(len(list[i]))
		binary.BigEndian.PutUint32(bytes4, length)
		buffer.Write(bytes4)
		buffer.Write(list[i])
	}
	return buffer.Bytes()
}

func decodeList(data []byte, prefetch bool) [][]byte {
	if prefetch {
		Prefetch(uintptr(unsafe.Pointer(&data[0])))
	}
	size := binary.BigEndian.Uint32(data[:4])
	data = data[4:]
	list := make([][]byte, size)
	for i := uint32(0); i < size; i++ {
		if prefetch {
			Prefetch(uintptr(unsafe.Pointer(&data[0])))
		}
		length := binary.BigEndian.Uint32(data[:4])
		data = data[4:]
		payload := make([]byte, length)
		copy(payload, data[:length])
		list[i] = payload
		data = data[length:]
	}
	return list
}

var dirtyMem = createTestBytes(1024)

func poisonCache(size int) byte {
	var target byte = 'a'
	for i := 0; i < size; i++ {
		if dirtyMem[i] < 'j' {
			target = (target+dirtyMem[i])%26 + 'a'
		}
	}
	return target
}

func createTestBytes(size int) []byte {
	data := make([]byte, size)
	data[0] = byte(int(uintptr(unsafe.Pointer(&data[0])))%26 + 'a')
	for i := 1; i < size; i++ {
		data[i]++
		if data[i] > 'z' {
			data[i] = 'a'
		}
	}
	return data
}

var benchList [][]byte

func init() {
	listSize := 128
	itemSize := 64
	for i := 0; i < listSize; i++ {
		benchList = append(benchList, createTestBytes(itemSize))
	}
}

func BenchmarkNoPrefetch(b *testing.B) {
	data := encodeList(benchList)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		poisonCache(len(dirtyMem))
		newlist := decodeList(data, false)
		_ = newlist
	}
}

func BenchmarkPrefetch(b *testing.B) {
	data := encodeList(benchList)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		poisonCache(len(dirtyMem))
		newlist := decodeList(data, true)
		_ = newlist
	}
}
