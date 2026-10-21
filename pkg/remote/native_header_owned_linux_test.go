package remote

import (
	"context"
	"encoding/binary"
	"reflect"
	"runtime"
	"strings"
	"syscall"
	"testing"

	"github.com/cloudwego/gopkg/protocol/ttheader"
)

func TestNativeHeaderStringsSurviveUnmappedFrameAndRecycle(t *testing.T) {
	for _, response := range []bool{false, true} {
		param := ttheader.EncodeParam{IntInfo: map[uint16]string{3: "caller", 9: "method"}, StrInfo: map[string]string{"meta": "owned-after-release", ttheader.GDPRToken: "acl-token", "\x00二进制": "\xff\x00value"}}
		wire, err := ttheader.EncodeToBytes(context.Background(), param)
		if err != nil {
			t.Fatal(err)
		}
		binary.BigEndian.PutUint32(wire, uint32(len(wire)-4))
		mapped, err := syscall.Mmap(-1, 0, len(wire), syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_PRIVATE|syscall.MAP_ANON)
		if err != nil {
			t.Fatal(err)
		}
		copy(mapped, wire)
		ti := newTransInfo().(*transInfo)
		headers := AccessHeaders(ti)
		headers.EnablePrimary(response)
		reader := &nativeByteBuffer{data: mapped}
		_, decodeErr := headers.DecodeTTHeader(context.Background(), reader)
		reader.Release(nil)
		unmapErr := syscall.Munmap(mapped)
		if decodeErr != nil || unmapErr != nil {
			ti.Recycle()
			t.Fatalf("decode=%v unmap=%v", decodeErr, unmapErr)
		}
		ints, strs := make(map[uint16]string), make(map[string]string)
		headers.RangeInt(func(k uint16, v string) bool { ints[k] = v; return true })
		headers.RangeString(func(k, v string) bool { strs[k] = v; return true })
		ti.Recycle()
		for i := 0; i < 32; i++ {
			fresh := newTransInfo().(*transInfo)
			h := AccessHeaders(fresh)
			h.EnablePrimary(response)
			h.SetString("meta", strings.Repeat("changed", i+1))
			fresh.Recycle()
		}
		for i := 0; i < 3; i++ {
			runtime.GC()
		}
		if !reflect.DeepEqual(ints, param.IntInfo) || !reflect.DeepEqual(strs, param.StrInfo) {
			t.Fatalf("escaped strings changed: ints=%v strs=%v", ints, strs)
		}
	}
}
