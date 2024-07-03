/*
 * Copyright 2021 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package loadbalance

import (
	"context"
	"fmt"
	"github.com/bytedance/gopkg/lang/fastrand"
	"github.com/cloudwego/kitex/pkg/loadbalance/newconsist"
	"math/rand"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/cloudwego/kitex/internal"
	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/discovery"
)

type testCtxKey int

const (
	keyCtxKey testCtxKey = iota
)

func getKey(ctx context.Context, request interface{}) string {
	if val, ok := ctx.Value(keyCtxKey).(string); ok {
		return val
	}
	return "1234"
}

func getRandomKey(ctx context.Context, request interface{}) string {
	key, ok := ctx.Value(keyCtxKey).(string)
	if !ok {
		return ""
	}
	return key
}

func getRandomString(r *rand.Rand, length int) string {
	var resBuilder strings.Builder
	resBuilder.Grow(length)
	corpus := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	for i := 0; i < length; i++ {
		resBuilder.WriteByte(corpus[r.Intn(len(corpus))])
	}
	return resBuilder.String()
}

func newTestConsistentHashOption() ConsistentHashOption {
	opt := NewConsistentHashOption(getKey)
	return opt
}

func TestNewConsistHashOption(t *testing.T) {
	opt := NewConsistentHashOption(getKey)
	test.Assert(t, opt.GetKey != nil)
	test.Assert(t, opt.VirtualFactor == 100)
	test.Assert(t, opt.Weighted)
}

func TestNewConsistBalancer(t *testing.T) {
	opt := ConsistentHashOption{}
	test.Panic(t, func() { NewConsistBalancer(opt) })
}

func TestConsistPicker_Next_Nil(t *testing.T) {
	opt := newTestConsistentHashOption()
	opt.GetKey = func(ctx context.Context, request interface{}) string {
		return ""
	}

	insList := []discovery.Instance{
		discovery.NewInstance("tcp", "addr1", 10, nil),
	}
	e := discovery.Result{
		Cacheable: false,
		CacheKey:  "",
		Instances: insList,
	}

	cb := NewConsistBalancer(opt)
	picker := cb.GetPicker(e)
	test.Assert(t, picker.Next(context.TODO(), nil) == nil)

	e = discovery.Result{
		Cacheable: false,
		CacheKey:  "",
		Instances: nil,
	}

	cb = NewConsistBalancer(newTestConsistentHashOption())
	picker = cb.GetPicker(e)
	test.Assert(t, picker.Next(context.TODO(), nil) == nil)
	test.Assert(t, cb.Name() == "consist")
}

// Replica related test
//func TestConsistPicker_Replica(t *testing.T) {
//	opt := NewConsistentHashOption(getKey)
//	opt.Replica = 1
//	opt.GetKey = func(ctx context.Context, request interface{}) string {
//		return "1234"
//	}
//	insList := makeNInstances(2, 10)
//	e := discovery.Result{
//		Cacheable: false,
//		CacheKey:  "",
//		Instances: insList,
//	}
//
//	cb := NewConsistBalancer(opt)
//	picker := cb.GetPicker(e)
//	first := picker.Next(context.TODO(), nil)
//	second := picker.Next(context.TODO(), nil)
//	test.Assert(t, first != second)
//}

// Replica related test
//func TestConsistPicker_Next_NoCache(t *testing.T) {
//	opt := newTestConsistentHashOption()
//	ins := discovery.NewInstance("tcp", "addr1", 10, nil)
//	insList := []discovery.Instance{
//		ins,
//	}
//	e := discovery.Result{
//		Cacheable: false,
//		CacheKey:  "",
//		Instances: insList,
//	}
//
//	cb := NewConsistBalancer(opt)
//	picker := cb.GetPicker(e)
//	test.Assert(t, picker.Next(context.TODO(), nil) == ins)
//	test.Assert(t, picker.Next(context.TODO(), nil) == nil)
//}

func TestConsistPicker_Next_NoCache_Consist(t *testing.T) {
	opt := newTestConsistentHashOption()
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
	opt.GetKey = func(ctx context.Context, request interface{}) string {
		v := ctx.Value("key")
		return v.(string)
	}

	cnt := make(map[string]int)
	for _, ins := range insList {
		cnt[ins.Address().String()] = 0
	}
	cnt["null"] = 0

	cb := NewConsistBalancer(opt)
	picker := cb.GetPicker(e)
	for i := 0; i < 100000; i++ {
		ctx := context.WithValue(context.Background(), "key", strconv.Itoa(i))
		if res := picker.Next(ctx, nil); res != nil {
			cnt[res.Address().String()]++
		} else {
			cnt["null"]++
		}
	}
	fmt.Println(cnt)

}

func TestConsistPicker_Next_Cache(t *testing.T) {
	opt := newTestConsistentHashOption()
	ins := discovery.NewInstance("tcp", "addr1", 10, nil)
	insList := []discovery.Instance{
		ins,
	}
	e := discovery.Result{
		Cacheable: true,
		CacheKey:  "4321",
		Instances: insList,
	}

	cb := NewConsistBalancer(opt)
	picker := cb.GetPicker(e)
	test.Assert(t, picker.Next(context.TODO(), nil) == ins)
}

func TestConsistPicker_Next_Cache_Consist(t *testing.T) {
	opt := newTestConsistentHashOption()
	insList := []discovery.Instance{
		discovery.NewInstance("tcp", "addr1", 10, nil),
		discovery.NewInstance("tcp", "addr2", 10, nil),
		discovery.NewInstance("tcp", "addr3", 10, nil),
		discovery.NewInstance("tcp", "addr4", 10, nil),
		discovery.NewInstance("tcp", "addr5", 10, nil),
		discovery.NewInstance("tcp", "addr6", 10, nil),
		discovery.NewInstance("tcp", "addr7", 10, nil),
		discovery.NewInstance("tcp", "addr8", 10, nil),
		discovery.NewInstance("tcp", "addr9", 10, nil),
	}
	e := discovery.Result{
		Cacheable: true,
		CacheKey:  "4321",
		Instances: insList,
	}

	cb := NewConsistBalancer(opt)
	picker := cb.GetPicker(e)
	ins := picker.Next(context.TODO(), nil)
	for i := 0; i < 100; i++ {
		picker := cb.GetPicker(e)
		test.Assert(t, picker.Next(context.TODO(), nil) == ins)
	}

	cb = NewConsistBalancer(opt)
	for i := 0; i < 100; i++ {
		picker := cb.GetPicker(e)
		test.Assert(t, picker.Next(context.TODO(), nil) == ins)
	}
}

func TestConsistBalance(t *testing.T) {
	opt := ConsistentHashOption{
		GetKey: func(ctx context.Context, request interface{}) string {
			return strconv.Itoa(fastrand.Intn(100000))
		},
		Replica:       0,
		VirtualFactor: 1000,
		Weighted:      false,
	}
	inss := makeNInstances(10, 10)

	m := make(map[discovery.Instance]int)
	e := discovery.Result{
		Cacheable: true,
		CacheKey:  "4321",
		Instances: inss,
	}

	cb := NewConsistBalancer(opt)
	for i := 0; i < 100000; i++ {
		picker := cb.GetPicker(e)
		ins := picker.Next(context.TODO(), nil)
		m[ins]++
		if p, ok := picker.(internal.Reusable); ok {
			p.Recycle()
		}
	}
}

func TestWeightedConsistBalance(t *testing.T) {
	opt := ConsistentHashOption{
		GetKey: func(ctx context.Context, request interface{}) string {
			return strconv.Itoa(fastrand.Intn(100000))
		},
		Replica:       0,
		VirtualFactor: 1000,
		Weighted:      true,
	}
	inss := makeNInstances(10, 10)

	m := make(map[discovery.Instance]int)
	e := discovery.Result{
		Cacheable: true,
		CacheKey:  "4321",
		Instances: inss,
	}

	cb := NewConsistBalancer(opt)
	for i := 0; i < 100000; i++ {
		picker := cb.GetPicker(e)
		ins := picker.Next(context.TODO(), nil)
		m[ins]++
	}
}

func TestConsistPicker_Reblance(t *testing.T) {
	opt := NewConsistentHashOption(getKey)
	insList := makeNInstances(10, 10)
	e := discovery.Result{
		Cacheable: true,
		CacheKey:  "4321",
		Instances: insList[:5],
	}

	ctx := context.Background()
	cb := NewConsistBalancer(opt)
	record := make(map[string]discovery.Instance)
	for i := 0; i < 10; i++ {
		picker := cb.GetPicker(e)
		key := strconv.Itoa(i)
		ctx = context.WithValue(ctx, keyCtxKey, key)
		record[key] = picker.Next(ctx, nil)
	}
	c := discovery.Change{
		Result: e,
		Added:  insList[5:],
	}
	cb.(Rebalancer).Rebalance(c)
	for i := 0; i < 10; i++ {
		picker := cb.GetPicker(e)
		key := strconv.Itoa(i)
		ctx = context.WithValue(ctx, keyCtxKey, key)
		res := picker.Next(ctx, nil)
		test.DeepEqual(t, record[key], res)
	}
}

func BenchmarkNewConsistPicker_NoCache(bb *testing.B) {
	n := 10
	balancer := NewConsistBalancer(newTestConsistentHashOption())
	ctx := context.Background()

	for i := 0; i < 4; i++ {
		bb.Run(fmt.Sprintf("%dins", n), func(b *testing.B) {
			inss := makeNInstances(n, 10)
			e := discovery.Result{
				Cacheable: false,
				CacheKey:  "",
				Instances: inss,
			}
			picker := balancer.GetPicker(e)
			picker.Next(ctx, nil)
			picker.(internal.Reusable).Recycle()
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				picker := balancer.GetPicker(e)
				// picker.Next(ctx, nil)
				if r, ok := picker.(internal.Reusable); ok {
					r.Recycle()
				}
			}
		})
		n *= 10
	}
}

func BenchmarkNewConsistPicker(bb *testing.B) {
	n := 10
	balancer := NewConsistBalancer(newTestConsistentHashOption())
	ctx := context.Background()

	for i := 0; i < 4; i++ {
		bb.Run(fmt.Sprintf("%dins", n), func(b *testing.B) {
			inss := makeNInstances(n, 10)
			e := discovery.Result{
				Cacheable: true,
				CacheKey:  "test",
				Instances: inss,
			}
			picker := balancer.GetPicker(e)
			picker.Next(ctx, nil)
			picker.(internal.Reusable).Recycle()
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				picker := balancer.GetPicker(e)
				picker.Next(ctx, nil)
				if r, ok := picker.(internal.Reusable); ok {
					r.Recycle()
				}
			}
		})
		n *= 10
	}
}

// BenchmarkConsistPicker_RandomDistributionKey
// BenchmarkConsistPicker_RandomDistributionKey/10ins
// BenchmarkConsistPicker_RandomDistributionKey/10ins-12         	 2417481	       508.9 ns/op	      48 B/op	       1 allocs/op
// BenchmarkConsistPicker_RandomDistributionKey/100ins
// BenchmarkConsistPicker_RandomDistributionKey/100ins-12        	 2140726	       534.6 ns/op	      48 B/op	       1 allocs/op
// BenchmarkConsistPicker_RandomDistributionKey/1000ins
// BenchmarkConsistPicker_RandomDistributionKey/1000ins-12       	 2848216.          407.7 ns/op	      48 B/op	       1 allocs/op
// BenchmarkConsistPicker_RandomDistributionKey/10000ins
// BenchmarkConsistPicker_RandomDistributionKey/10000ins-12      	 2701766	       492.7 ns/op	      48 B/op	       1 allocs/op
func BenchmarkConsistPicker_RandomDistributionKey(b *testing.B) {
	n := 10
	balancer := NewConsistBalancer(NewConsistentHashOption(getRandomKey))

	for i := 0; i < 4; i++ {
		b.Run(fmt.Sprintf("%dins", n), func(b *testing.B) {
			r := rand.New(rand.NewSource(int64(n)))
			inss := makeNInstances(n, 10)
			e := discovery.Result{
				Cacheable: true,
				CacheKey:  "test",
				Instances: inss,
			}
			b.ReportAllocs()
			b.ResetTimer()
			picker := balancer.GetPicker(e)
			ctx := context.WithValue(context.Background(), keyCtxKey, getRandomString(r, 30))
			picker.Next(ctx, nil)
			picker.(internal.Reusable).Recycle()
			for j := 0; j < b.N; j++ {
				ctx = context.WithValue(context.Background(), keyCtxKey, getRandomString(r, 30))
				picker = balancer.GetPicker(e)
				picker.Next(ctx, nil)
				if toRecycle, ok := picker.(internal.Reusable); ok {
					toRecycle.Recycle()
				}
			}
		})
		n *= 10
	}
}

func BenchmarkRebalance(bb *testing.B) {
	weight := 10
	nums := 10000

	for n := 0; n < 1; n++ {
		bb.Run(fmt.Sprintf("consist-remove-%d", nums), func(b *testing.B) {
			insList := makeNInstances(nums, weight)
			e := discovery.Result{
				Cacheable: true,
				CacheKey:  "",
				Instances: insList,
			}
			newConsist := newconsist.NewConsistInfo(e, newconsist.ConsistInfoConfig{
				VirtualFactor: 100,
				Weighted:      true,
			})

			b.ReportAllocs()
			b.ResetTimer()
			change := discovery.Change{
				Result:  e,
				Added:   nil,
				Updated: nil,
			}
			removed := []discovery.Instance{insList[0]}
			for i := 0; i < nums; i++ {
				e.Instances = insList[i+1:]
				removed[0] = insList[i]
				change.Result = e
				change.Removed = removed
				newConsist.Rebalance(change)
			}
			runtime.GC()
		})

		bb.Run(fmt.Sprintf("consist-add-%d", nums), func(b *testing.B) {
			insList := makeNInstances(nums, weight)
			e := discovery.Result{
				Cacheable: true,
				CacheKey:  "",
				Instances: insList,
			}
			newConsist := newconsist.NewConsistInfo(e, newconsist.ConsistInfoConfig{
				VirtualFactor: 100,
				Weighted:      true,
			})

			b.ReportAllocs()
			b.ResetTimer()
			change := discovery.Change{
				Result:  e,
				Added:   nil,
				Updated: nil,
			}
			added := []discovery.Instance{insList[0]}
			for i := 0; i < nums; i++ {
				e.Instances = insList[:i+1]
				added[0] = insList[i]
				change.Result = e
				change.Added = added
				newConsist.Rebalance(change)
			}
			runtime.GC()
		})
		nums *= 10
	}

}

func BenchmarkNewConsistInfo(b *testing.B) {
	weight := 10
	nums := 10
	for n := 0; n < 4; n++ {
		b.Run(fmt.Sprintf("new-consist-%d", nums), func(b *testing.B) {
			insList := makeNInstances(nums, weight)
			e := discovery.Result{
				Cacheable: false,
				CacheKey:  "",
				Instances: insList,
			}
			b.ResetTimer()
			b.ReportAllocs()
			newConsist := newconsist.NewConsistInfo(e, newconsist.ConsistInfoConfig{
				VirtualFactor: 100,
				Weighted:      true,
			})
			_ = newConsist
		})
		nums *= 10
	}
}
