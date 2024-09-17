package container

import (
	"sync"
	"sync/atomic"
)

func NewQueue[ValueType any]() *Queue[ValueType] {
	return &Queue[ValueType]{
		L: &sync.Mutex{},
	}
}

func NewQueueWithLocker[ValueType any](locker sync.Locker) *Queue[ValueType] {
	return &Queue[ValueType]{
		L: locker,
	}
}

type queueNode[ValueType any] struct {
	val  ValueType
	next *queueNode[ValueType]
}

func (n *queueNode[ValueType]) reset() {
	var nilVal ValueType
	n.val = nilVal
	n.next = nil
}

type Queue[ValueType any] struct {
	L    sync.Locker
	read *queueNode[ValueType]
	head *queueNode[ValueType]
	tail *queueNode[ValueType]
	size int64

	nodePool sync.Pool
}

func (q *Queue[ValueType]) Size() int {
	return int(atomic.LoadInt64(&q.size))
}

func (q *Queue[ValueType]) Get() (val ValueType, ok bool) {
Start:
	// fast path
	if q.read != nil {
		node := q.read
		val = node.val
		q.read = node.next
		atomic.AddInt64(&q.size, -1)

		// recycle node
		node.reset()
		q.nodePool.Put(node)
		return val, true
	}
	// slow path
	q.L.Lock()
	if q.head == nil {
		q.L.Unlock()
		return val, false
	}
	// single read
	if q.head == q.tail {
		node := q.head
		val = node.val
		q.head = nil
		q.tail = nil
		atomic.AddInt64(&q.size, -1)
		q.L.Unlock()

		// recycle node
		node.reset()
		q.nodePool.Put(node)
		return val, true
	}
	// batch read into q.read
	q.read = q.head
	q.head = nil
	q.tail = nil
	q.L.Unlock()
	goto Start
}

func (q *Queue[ValueType]) Add(val ValueType) {
	q.L.Lock()

	var node *queueNode[ValueType]
	v := q.nodePool.Get()
	if v == nil {
		node = new(queueNode[ValueType])
	} else {
		node = v.(*queueNode[ValueType])
	}
	node.val = val
	if q.tail == nil {
		q.head = node
		q.tail = node
	} else {
		q.tail.next = node
		q.tail = q.tail.next
	}
	atomic.AddInt64(&q.size, 1)
	q.L.Unlock()
}
