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

func NewObjectPool(idleTimeout time.Duration) *ObjectPool {
	s := new(ObjectPool)
	s.idleTimeout = idleTimeout
	s.objects = make(map[string]*Stack[objectItem])
	go s.cleaning()
	return s
}

type ObjectPool struct {
	L           sync.Mutex // STW
	objects     map[string]*Stack[objectItem]
	idleTimeout time.Duration
	closed      int32
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
	cleanInternal := time.Second
	for atomic.LoadInt32(&s.closed) == 0 {
		time.Sleep(cleanInternal)

		now := time.Now()
		s.L.Lock()
		// update cleanInternal
		objSize := 0
		for _, stk := range s.objects {
			objSize += stk.Size()
		}
		cleanInternal = time.Second + time.Duration(objSize)*time.Millisecond*10
		if cleanInternal > time.Second*10 {
			cleanInternal = time.Second * 10
		}
		// clean objects
		for key, stk := range s.objects {
			deleted := 0
			var oldest *time.Time
			klog.Infof("object[%s] pool cleaning %d objects", key, stk.Size())
			stk.RangeDelete(func(o objectItem) (deleteNode bool, continueRange bool) {
				if oldest == nil {
					oldest = &o.lastActive
				}
				if o.object == nil {
					deleted++
					return true, true
				}

				// RangeDelete start from the stack bottom
				// we assume that the values on the top of last valid value are all valid
				if now.Sub(o.lastActive) < s.idleTimeout {
					return false, false
				}
				deleted++
				err := o.object.Close()
				klog.Infof("object is invalid: lastActive=%s, closedErr=%v", o.lastActive.String(), err)
				return true, true
			})
			if oldest != nil {
				klog.Infof("object[%s] pool deleted %d objects, oldest=%s", key, deleted, oldest.String())
			}
		}
		s.L.Unlock()
	}
}
