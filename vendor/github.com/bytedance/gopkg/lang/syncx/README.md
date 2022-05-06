# syncx

## 1. syncx.Pool
### Defects of sync.Pool
1. Get/Put func is unbalanced on P.
    1. Usually Get and Put are not fired on the same goroutine, which will result in a difference in the fired times of Get and Put on each P. The difference part will trigger the steal operation(using CAS).
2. poolLocal.private has only one.
    1. Get/Put operations are very frequent. In most cases, Get/Put needs to operate poolLocal.shared (using CAS) instead of the lock-free poolLocal.private.
3. Frequent GC.
    1. sync.Pool is designed to serve objects with a short life cycle, but we just want to reuse, don’t want frequent GC.

### Optimize of syncx.Pool
1. Transform poolLocal.private into an array to increase the frequency of poolLocal.private use.
2. Batch operation poolLocal.shared, reduce CAS calls.
3. Allow No GC.

### Not Recommended
1. A single object is too large.
   1. syncx.Pool permanently holds up to runtime.GOMAXPROC(0)*256 reusable objects.
   2. For example, under a 4-core docker, a 4KB object will cause about 4MB of memory usage.
   3. please evaluate it yourself.

### Performance
| benchmark | sync ns/op | syncx ns/op | delta |
| :---------- | :-----------: | :-----------: | :-----------: |
| BenchmarkSyncPoolParallel-4(p=16) | 877 | 620 | -29.30% |
| BenchmarkSyncPoolParallel-4(p=1024) | 49385 | 33465 | -32.24% |
| BenchmarkSyncPoolParallel-4(p=4096) | 255671 | 149522 | -41.52% |

### Example
```go
var pool = &syncx.Pool{
	New: func() interface{} {
		return &struct{}
	},
	NoGC: true,
}

func getput() {
	var obj = pool.Get().(*struct)
	pool.Put(obj)
}
```

## 2. syncx.RWMutex
