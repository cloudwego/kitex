package newconsist

import (
	"fmt"
	"github.com/cloudwego/kitex/internal/test"
	"testing"
)

func TestAllocator(t *testing.T) {
	size := 1000
	a := NewAllocator(size)

	num := 100
	objs := make([]*virtualNode, num)
	for i := 0; i < num; i++ {
		objs[i] = a.Alloc()
		objs[i].value = uint64(i)
	}

	test.Assert(t, int(a.read) == num, fmt.Sprintf("expected %d, got %d", num, a.read))

	//toFree := num - 10
	//for i := 0; i < num-10; i++ {
	//	objs[i].free()
	//}
	//freed := a.Free()
	//test.Assert(t, freed == uint32(toFree), fmt.Sprintf("expected %d, got %d", toFree, freed))
	//test.Assert(t, int(a.size) == size-toFree)
	//test.Assert(t, int(a.read) == 10)
	//
	//for i := 0; i < 10; i++ {
	//	test.Assert(t, objs[num-10+i] == &a.cache[i+toFree])
	//}
}

func TestSlice(t *testing.T) {
	num := 10
	objectSlice := make([]virtualNode, num)
	for i := 0; i < num; i++ {
		objectSlice[i].value = uint64(i)
	}

	idx := 3
	pointer := &objectSlice[idx]
	oldValue := pointer.value
	for i := 1; i < num; i++ {
		objectSlice[i-1] = objectSlice[i]
	}
	objectSlice = objectSlice[:num]
	newValue := pointer.value
	test.Assert(t, newValue == oldValue)
}
