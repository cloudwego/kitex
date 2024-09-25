package container

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/cloudwego/kitex/pkg/klog"
)

type Object interface {
	IsInvalid() bool
	Close() error
}

func NewObjectPool(cleanInternal time.Duration) *ObjectPool {
	s := new(ObjectPool)
	s.objects = map[string]*Stack[Object]{}
	s.cleanInternal = cleanInternal
	go s.cleaning()
	return s
}

type ObjectPool struct {
	L             sync.Mutex // STW
	objects       map[string]*Stack[Object]
	cleanInternal time.Duration
	closed        int32
}

func (s *ObjectPool) Push(key string, o Object) {
	s.L.Lock()
	stk := s.objects[key]
	if stk == nil {
		stk = NewStack[Object]()
		s.objects[key] = stk
	}
	s.L.Unlock()
	stk.Push(o)
}

func (s *ObjectPool) Pop(key string) Object {
	s.L.Lock()
	stk := s.objects[key]
	s.L.Unlock()
	if stk == nil {
		return nil
	}
	o, _ := stk.Pop()
	return o
}

func (s *ObjectPool) Close() {
	atomic.CompareAndSwapInt32(&s.closed, 0, 1)
}

func (s *ObjectPool) cleaning() {
	for atomic.LoadInt32(&s.closed) == 0 {
		time.Sleep(s.cleanInternal)

		s.L.Lock()
		for _, stk := range s.objects {
			stk.RangeDelete(func(o Object) (deleteNode bool, continueRange bool) {
				if o == nil {
					return true, true
				}

				// RangeDelete start from the stack bottom
				// we assume that the values on the top of last valid value are all valid
				if !o.IsInvalid() {
					return false, false
				}
				klog.Infof("object is invalid: %v, closing", o)
				_ = o.Close()
				return true, true
			})
		}
		s.L.Unlock()
	}
}
