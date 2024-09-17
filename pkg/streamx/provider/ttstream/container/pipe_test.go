package container

import (
	"sync"
	"testing"
)

func TestPipeline(t *testing.T) {
	pipe := NewPipe[int]()
	var recv int
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		items := make([]int, 10)
		for {
			n, err := pipe.Read(items)
			if err != nil {
				return
			}
			for i := 0; i < n; i++ {
				recv += items[i]
			}
		}
	}()
	round := 10000
	itemsPerRound := []int{1, 1, 1, 1, 1}
	for i := 0; i < round; i++ {
		pipe.Write(itemsPerRound...)
	}
	pipe.Close()
	wg.Wait()
	if recv != len(itemsPerRound)*round {
		t.Fatalf("expect %d items, got %d", len(itemsPerRound)*round, recv)
	}
}

func BenchmarkPipeline(b *testing.B) {
	pipe := NewPipe[int]()
	readCache := make([]int, 8)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		for j := 0; j < len(readCache); j++ {
			go pipe.Write(1)
		}
		total := 0
		for total < len(readCache) {
			n, _ := pipe.Read(readCache)
			total += n
		}
	}
}
