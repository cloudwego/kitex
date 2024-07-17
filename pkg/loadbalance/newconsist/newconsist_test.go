package newconsist

import (
	"fmt"
	"github.com/bytedance/gopkg/util/xxhash3"
	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/discovery"
	"math/rand"
	"runtime"
	"strconv"
	"sync"
	"testing"
	"time"
)

func TestNewSkipList(t *testing.T) {
	s := newSkipList()
	dataCnt := 10000
	for i := 0; i < dataCnt; i++ {
		s.Insert(&virtualNode{realNode: nil, value: uint64(i)})
		test.Assert(t, s.Search(uint64(i)))
	}
	totalCnt := 0
	for i := 0; i < s.totalLevel; i++ {
		currCnt := countLevel(s, i)
		totalCnt += currCnt
	}
	fmt.Printf("totalCnt: %d, ratio: %f\n", totalCnt, float64(totalCnt)/float64(dataCnt))
	for i := 0; i < dataCnt; i++ {
		s.Delete(uint64(i))
		currCnt := countLevel(s, 0)
		test.Assert(t, currCnt == dataCnt-i-1)
	}
	s.FindGreater(0)
	test.Assert(t, countLevel(s, 0) == 0)
}

func TestFuzzSkipList(t *testing.T) {
	rand.Seed(time.Now().UnixNano())
	sl := newSkipList()

	vs := make([]uint64, 1000)
	// 插入1000个随机值
	for i := 0; i < 1000; i++ {
		value := rand.Uint64() % 10000
		vs[i] = value
		node := &virtualNode{nil, value, nil}
		sl.Insert(node)
	}

	// 搜索1000个随机值
	for i := 0; i < 1000; i++ {
		found := sl.Search(vs[i])
		test.Assert(t, found)
	}

	// 删除500个随机值
	for i := 0; i < 500; i++ {
		value := vs[i]
		sl.Delete(value)
	}
	for i := 500; i < 1000; i++ {
		found := sl.Search(vs[i])
		test.Assert(t, found)
	}
}

func TestGetVirtualNodeHash(t *testing.T) {
	insList := []discovery.Instance{
		discovery.NewInstance("tcp", "addr1", 10, nil),
		discovery.NewInstance("tcp", "addr2", 10, nil),
		discovery.NewInstance("tcp", "addr3", 10, nil),
		discovery.NewInstance("tcp", "addr4", 10, nil),
		discovery.NewInstance("tcp", "addr5", 10, nil),
	}
	e := discovery.Result{
		Cacheable: false,
		CacheKey:  "",
		Instances: insList,
	}
	info := NewConsistInfo(e, ConsistInfoConfig{VirtualFactor: 100, Weighted: true})
	info.getVirtualNodeHash([]byte{1, 2, 3}, 1)
}

func Test_getConsistentResult(t *testing.T) {
	insList := []discovery.Instance{
		discovery.NewInstance("tcp", "addr1", 100, nil),
		discovery.NewInstance("tcp", "addr2", 100, nil),
		discovery.NewInstance("tcp", "addr3", 100, nil),
		discovery.NewInstance("tcp", "addr4", 100, nil),
		discovery.NewInstance("tcp", "addr5", 100, nil),
	}
	e := discovery.Result{
		Cacheable: false,
		CacheKey:  "",
		Instances: insList,
	}
	info := NewConsistInfo(e, ConsistInfoConfig{VirtualFactor: 100, Weighted: true})
	newInsList := make([]discovery.Instance, len(insList)-1)
	copy(newInsList, insList[1:])
	newResult := discovery.Result{
		Cacheable: false,
		CacheKey:  "",
		Instances: newInsList,
	}
	change := discovery.Change{
		Result:  newResult,
		Added:   nil,
		Removed: []discovery.Instance{insList[0]},
		Updated: nil,
	}
	info.Rebalance(change)
	cnt := make(map[string]int)
	for _, ins := range insList {
		cnt[ins.Address().String()] = 0
	}
	cnt["null"] = 0
	for i := 0; i < 100000; i++ {
		if res := info.BuildConsistentResult(xxhash3.HashString(strconv.Itoa(i))); res != nil {
			cnt[res.Address().String()]++
		} else {
			cnt["null"]++
		}
	}
	fmt.Println(cnt)
}

func TestRebalance(t *testing.T) {
	nums := 1000
	insList := make([]discovery.Instance, 0, nums)
	for i := 0; i < nums; i++ {
		insList = append(insList, discovery.NewInstance("tcp", "addr"+strconv.Itoa(i), 100, nil))
	}
	e := discovery.Result{
		Cacheable: false,
		CacheKey:  "",
		Instances: insList,
	}
	newConsist := NewConsistInfo(e, ConsistInfoConfig{
		VirtualFactor: 100,
		Weighted:      true,
	})
	//for i := 0; i < nums; i++ {
	//	_, all := searchRealNode(newConsist, &realNode{insList[i]})
	//	// should find all virtual node
	//	test.Assert(t, all)
	//}
	for i := 0; i < nums; i++ {
		e.Instances = insList[i+1:]
		change := discovery.Change{
			Result:  e,
			Added:   nil,
			Removed: []discovery.Instance{insList[i]},
			Updated: nil,
		}
		newConsist.Rebalance(change)

		//one, _ := searchRealNode(newConsist, &realNode{insList[i]})
		//// no virtual node should be found
		//test.Assert(t, !one)
		//
		//for j := i + 1; j < nums; j++ {
		//	_, all := searchRealNode(newConsist, &realNode{insList[j]})
		//	// should find all virtual node
		//	test.Assert(t, all)
		//}
		newConsist.BuildConsistentResult(xxhash3.HashString("1"))
	}
}

var (
	ciMap sync.Map
)

func TestRebalanceMemory(t *testing.T) {
	nums := 10000
	insList := make([]discovery.Instance, 0, nums)
	for i := 0; i < nums; i++ {
		insList = append(insList, discovery.NewInstance("tcp", "addr"+strconv.Itoa(i), 10, nil))
	}
	e := discovery.Result{
		Cacheable: false,
		CacheKey:  "",
		Instances: insList,
	}
	newConsist := NewConsistInfo(e, ConsistInfoConfig{
		VirtualFactor: 100,
		Weighted:      true,
	})
	for i := 0; i < nums; i++ {
		_, all := searchRealNode(newConsist, &realNode{insList[i]})
		// should find all virtual node
		test.Assert(t, all)
	}
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	t.Logf("Before Delete node, heap allocation: %f Mb, heap objects in use: %d\n", mb(ms.HeapAlloc), ms.HeapObjects)

	s := newConsist.virtualNodes
	totalCnt := 0
	for i := 0; i < s.totalLevel; i++ {
		currCnt := countLevel(s, i)
		totalCnt += currCnt
	}
	t.Logf("Before Delete node, total node cnt: %d\n", totalCnt)

	for i := 0; i < 5000; i++ {
		e.Instances = insList[i+1:]
		change := discovery.Change{
			Result:  e,
			Added:   nil,
			Removed: []discovery.Instance{insList[i]},
			Updated: nil,
		}
		newConsist.Rebalance(change)
	}
	s = newConsist.virtualNodes
	totalCnt = 0
	for i := 0; i < s.totalLevel; i++ {
		currCnt := countLevel(s, i)
		totalCnt += currCnt
	}
	t.Logf("After Delete node, total node cnt: %d\n", totalCnt)

	runtime.ReadMemStats(&ms)
	t.Logf("After Delete node, heap allocation: %f Mb, heap objects in use: %d\n", mb(ms.HeapAlloc), ms.HeapObjects)

	ciMap.Store("a", newConsist)
	runtime.GC()
	runtime.ReadMemStats(&ms)
	t.Logf("After GC, heap allocation: %f Mb, heap objects in use: %d\n", mb(ms.HeapAlloc), ms.HeapObjects)
}

func TestRebalanceDupilicate(t *testing.T) {
	nums := 1000
	duplicate := 10
	insList := make([]discovery.Instance, 0, nums)
	for i := 0; i < nums; i++ {
		insList = append(insList, discovery.NewInstance("tcp", "addr"+strconv.Itoa(i), 10, nil))
	}
	e := discovery.Result{
		Cacheable: false,
		CacheKey:  "",
		Instances: insList,
	}
	newConsist := NewConsistInfo(e, ConsistInfoConfig{
		VirtualFactor: 100,
		Weighted:      true,
	})

	for i := 0; i < nums; i++ {
		e.Instances = insList[i+1:]
		change := discovery.Change{
			Result:  e,
			Added:   nil,
			Removed: []discovery.Instance{insList[i]},
			Updated: nil,
		}
		for j := 0; j < duplicate; j++ {
			newConsist.Rebalance(change)
			one, _ := searchRealNode(newConsist, &realNode{insList[i]})
			// no virtual node should be found
			test.Assert(t, !one)
		}
	}
}

func mb(byteSize uint64) float32 {
	return float32(byteSize) / float32(1024*1024)
}
