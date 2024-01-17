package allocator_test

import (
	"fmt"
	"runtime"
	"sync"
	"testing"
)

func BenchmarkReuseMap(b *testing.B) {
	var mapSizeCases = []int{10, 50, 100, 200}
	var reuseCases = []bool{
		//false,
		true,
	}
	for _, reuse := range reuseCases {
		for _, mapsize := range mapSizeCases {
			var testKVPairs []string
			testKVPairs = make([]string, mapsize*2)
			for i := 0; i < mapsize*2; i += 2 {
				testKVPairs[i] = fmt.Sprintf("key=%d", i)
				testKVPairs[i+1] = fmt.Sprintf("val=%d", i)
			}

			b.Run(fmt.Sprintf("ReuseMap=%v,MapSize=%d", reuse, mapsize), func(b *testing.B) {
				var mapPool sync.Pool
				b.ReportAllocs()
				b.ResetTimer()
				b.RunParallel(func(pb *testing.PB) {
					var m map[string]string
					for pb.Next() {
						if reuse {
							if item := mapPool.Get(); item == nil {
								m = make(map[string]string)
							} else {
								m = item.(map[string]string)
							}
						} else {
							m = make(map[string]string)
						}

						for i := 0; i < len(testKVPairs); i += 2 {
							m[testKVPairs[i]] = testKVPairs[i+1]
						}

						if reuse {
							tmp := m
							runtime.SetFinalizer(&tmp, func(mm *map[string]string) {
								for k := range *mm {
									delete(*mm, k)
								}
								mapPool.Put(*mm)
							})
						}
					}
				})
			})
		}
	}
}
