package container

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/cloudwego/kitex/pkg/klog"
)

type Object interface {
	Close() error
}

type objectItem struct {
	object     Object
	lastActive time.Time
}

func NewObjectPool(idleTimeout, cleanInternal time.Duration) *ObjectPool {
	s := new(ObjectPool)
	s.idleTimeout = idleTimeout
	s.cleanInternal = cleanInternal
	s.objects = make(map[string]*Stack[objectItem])
	go s.cleaning()
	return s
}

type ObjectPool struct {
	L             sync.Mutex // STW
	objects       map[string]*Stack[objectItem]
	idleTimeout   time.Duration
	cleanInternal time.Duration
	closed        int32
}

func (s *ObjectPool) Push(key string, o Object) {
	s.L.Lock()
	stk := s.objects[key]
	if stk == nil {
		stk = NewStack[objectItem]()
		s.objects[key] = stk
	}
	s.L.Unlock()
	stk.Push(objectItem{object: o, lastActive: time.Now()})
}

func (s *ObjectPool) Pop(key string) Object {
	s.L.Lock()
	stk := s.objects[key]
	s.L.Unlock()
	if stk == nil {
		return nil
	}
	o, ok := stk.Pop()
	if !ok {
		return nil
	}
	return o.object
}

func (s *ObjectPool) Close() {
	atomic.CompareAndSwapInt32(&s.closed, 0, 1)
}

func (s *ObjectPool) cleaning() {
	for atomic.LoadInt32(&s.closed) == 0 {
		time.Sleep(s.cleanInternal)

		now := time.Now()
		s.L.Lock()
		for _, stk := range s.objects {
			stk.RangeDelete(func(o objectItem) (deleteNode bool, continueRange bool) {
				if o.object == nil {
					return true, true
				}

				// RangeDelete start from the stack bottom
				// we assume that the values on the top of last valid value are all valid
				if now.Sub(o.lastActive) < s.idleTimeout {
					return false, false
				}
				err := o.object.Close()
				klog.Infof("object is invalid: lastActive=%s, closedErr=%v", o.lastActive, err)
				return true, true
			})
		}
		s.L.Unlock()
	}
}
