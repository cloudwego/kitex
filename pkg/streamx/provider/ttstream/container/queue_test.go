package container

import (
	"sync"
	"testing"
)

func TestQueue(t *testing.T) {
	locker := sync.RWMutex{}
	q := NewQueueWithLocker[int](&locker)
	round := 100000
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		for i := 0; i < round; i++ {
			q.Add(1)
		}
	}()
	for sum := 0; sum < round; {
		v, ok := q.Get()
		if ok {
			sum += v
		}
	}
}
