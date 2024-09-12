package ttstream

import (
	"sync"
	"time"
)

type Object interface {
	Available() bool
	Close() error
}

func newScavenger() *scavenger {
	s := new(scavenger)
	go s.Cleaning()
	return s
}

type scavenger struct {
	sync.RWMutex
	objects []Object
}

func (s *scavenger) Add(o Object) {
	s.Lock()
	s.objects = append(s.objects, o)
	s.Unlock()
}

func (s *scavenger) Cleaning() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for range ticker.C {
		s.RLock()
		for _, o := range s.objects {
			if !o.Available() {
				_ = o.Close()
			}
		}
		s.RUnlock()
	}
}
