<p align="center">
  <img src="https://raw.githubusercontent.com/zhangyunhao116/public-data/master/skipmap-logo.png"/>
</p>

## Introduction

skipmap is a high-performance concurrent map based on skip list. In typical pattern(one million operations, 90%LOAD 9%STORE 1%DELETE), the skipmap up to 3x ~ 10x faster than the built-in sync.Map.

The main idea behind the skipmap is [A Simple Optimistic Skiplist Algorithm](<https://people.csail.mit.edu/shanir/publications/LazySkipList.pdf>).

Different from the sync.Map, the items in the skipmap are always sorted, and the `Load` and `Range` operations are wait-free (A goroutine is guaranteed to complete a operation as long as it keeps taking steps, regardless of the activity of other goroutines).



## Features

- Concurrent safe API with high-performance.
- Wait-free Load and Range operations.
- Sorted items.



## When should you use skipmap

In these situations, `skipmap` is better

- **Sorted elements is needed**.
- **Concurrent calls multiple operations**. such as use both `Load` and `Store` at the same time.

In these situations, `sync.Map` is better

- Only one goroutine access the map for most of the time, such as insert a batch of elements and then use only `Load` (use built-in map is even better).



## QuickStart

```go
package main

import (
	"fmt"

	"github.com/bytedance/gopkg/collection/skipmap"
)

func main() {
	l := skipmap.NewInt()

	for _, v := range []int{10, 12, 15} {
		l.Store(v, v+100)
	}

	v, ok := l.Load(10)
	if ok {
		fmt.Println("skipmap load 10 with value ", v)
	}

	l.Range(func(key int, value interface{}) bool {
		fmt.Println("skipmap range found ", key, value)
		return true
	})

	l.Delete(15)
	fmt.Printf("skipmap contains %d items\r\n", l.Len())
}

```



## Benchmark

Go version: go1.16.2 linux/amd64

CPU: AMD 3700x(8C16T), running at 3.6GHz

OS: ubuntu 18.04

MEMORY: 16G x 2 (3200MHz)

![benchmark](https://raw.githubusercontent.com/zhangyunhao116/public-data/master/skipmap-benchmark.png)

```shell
$ go test -run=NOTEST -bench=. -benchtime=100000x -benchmem -count=20 -timeout=60m  > x.txt
$ benchstat x.txt
```

```
name                                            time/op
Int64/Store/skipmap-16                           158ns ±12%
Int64/Store/sync.Map-16                          700ns ± 4%
Int64/Load50Hits/skipmap-16                     10.1ns ±14%
Int64/Load50Hits/sync.Map-16                    14.8ns ±23%
Int64/30Store70Load/skipmap-16                  50.6ns ±20%
Int64/30Store70Load/sync.Map-16                  592ns ± 7%
Int64/1Delete9Store90Load/skipmap-16            27.5ns ±13%
Int64/1Delete9Store90Load/sync.Map-16            480ns ± 4%
Int64/1Range9Delete90Store900Load/skipmap-16    34.2ns ±26%
Int64/1Range9Delete90Store900Load/sync.Map-16   1.00µs ±12%
String/Store/skipmap-16                          171ns ±15%
String/Store/sync.Map-16                         873ns ± 4%
String/Load50Hits/skipmap-16                    21.3ns ±38%
String/Load50Hits/sync.Map-16                   19.9ns ±12%
String/30Store70Load/skipmap-16                 75.6ns ±16%
String/30Store70Load/sync.Map-16                 726ns ± 5%
String/1Delete9Store90Load/skipmap-16           34.3ns ±20%
String/1Delete9Store90Load/sync.Map-16           584ns ± 5%
String/1Range9Delete90Store900Load/skipmap-16   41.0ns ±21%
String/1Range9Delete90Store900Load/sync.Map-16  1.17µs ± 8%

name                                            alloc/op
Int64/Store/skipmap-16                            112B ± 0%
Int64/Store/sync.Map-16                           128B ± 0%
Int64/Load50Hits/skipmap-16                      0.00B     
Int64/Load50Hits/sync.Map-16                     0.00B     
Int64/30Store70Load/skipmap-16                   33.0B ± 0%
Int64/30Store70Load/sync.Map-16                  81.2B ±11%
Int64/1Delete9Store90Load/skipmap-16             10.0B ± 0%
Int64/1Delete9Store90Load/sync.Map-16            57.9B ± 5%
Int64/1Range9Delete90Store900Load/skipmap-16     10.0B ± 0%
Int64/1Range9Delete90Store900Load/sync.Map-16     261B ±17%
String/Store/skipmap-16                           144B ± 0%
String/Store/sync.Map-16                          152B ± 0%
String/Load50Hits/skipmap-16                     15.0B ± 0%
String/Load50Hits/sync.Map-16                    15.0B ± 0%
String/30Store70Load/skipmap-16                  54.0B ± 0%
String/30Store70Load/sync.Map-16                 96.9B ±12%
String/1Delete9Store90Load/skipmap-16            27.0B ± 0%
String/1Delete9Store90Load/sync.Map-16           74.2B ± 4%
String/1Range9Delete90Store900Load/skipmap-16    27.0B ± 0%
String/1Range9Delete90Store900Load/sync.Map-16    257B ±10%

name                                            allocs/op
Int64/Store/skipmap-16                            3.00 ± 0%
Int64/Store/sync.Map-16                           4.00 ± 0%
Int64/Load50Hits/skipmap-16                       0.00     
Int64/Load50Hits/sync.Map-16                      0.00     
Int64/30Store70Load/skipmap-16                    0.00     
Int64/30Store70Load/sync.Map-16                   1.00 ± 0%
Int64/1Delete9Store90Load/skipmap-16              0.00     
Int64/1Delete9Store90Load/sync.Map-16             0.00     
Int64/1Range9Delete90Store900Load/skipmap-16      0.00     
Int64/1Range9Delete90Store900Load/sync.Map-16     0.00     
String/Store/skipmap-16                           4.00 ± 0%
String/Store/sync.Map-16                          5.00 ± 0%
String/Load50Hits/skipmap-16                      1.00 ± 0%
String/Load50Hits/sync.Map-16                     1.00 ± 0%
String/30Store70Load/skipmap-16                   1.00 ± 0%
String/30Store70Load/sync.Map-16                  2.00 ± 0%
String/1Delete9Store90Load/skipmap-16             1.00 ± 0%
String/1Delete9Store90Load/sync.Map-16            1.00 ± 0%
String/1Range9Delete90Store900Load/skipmap-16     1.00 ± 0%
String/1Range9Delete90Store900Load/sync.Map-16    1.00 ± 0%
```