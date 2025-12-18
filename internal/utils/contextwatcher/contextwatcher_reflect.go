package contextwatcher

import (
	"context"
	"reflect"
	"runtime"
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/cloudwego/kitex/pkg/gofunc"
)

var global = newContextWatcherReflect(runtime.GOMAXPROCS(0)*3/2, 64, 1)

// contextWatcherReflect uses reflect.Select to monitor multiple contexts efficiently.
// Instead of creating one goroutine per context, it uses a fixed number of shards,
// each running a single goroutine that monitors multiple contexts using reflect.Select.
type contextWatcherReflect struct {
	shardCapacity  int
	signalCapacity int

	shards         atomic.Pointer[[]*reflectShard]
	expandMutex    sync.Mutex
	ctxShards      sync.Map // map[context.Context]int - tracks which shard each context belongs to
	activeContexts atomic.Int64
}

func RegisterContext(ctx context.Context, callback func(context.Context)) {
	global.RegisterContext(ctx, callback)
}

func DeregisterContext(ctx context.Context) {
	global.DeregisterContext(ctx)
}

// newContextWatcherReflect creates a new contextWatcherReflect with default number of shards.
func newContextWatcherReflect(shardNum, shardCapacity, signalCapacity int) *contextWatcherReflect {
	w := &contextWatcherReflect{
		shardCapacity:  shardCapacity,
		signalCapacity: signalCapacity,
	}

	shards := make([]*reflectShard, shardNum)
	for i := 0; i < shardNum; i++ {
		shards[i] = newReflectShard(w, shardCapacity, signalCapacity)
		go shards[i].run()
	}
	w.shards.Store(&shards)

	return w
}

func (w *contextWatcherReflect) getShard(ctx context.Context) (shard *reflectShard, exist, fallback bool) {
	shards := *w.shards.Load()
	n := len(shards)
	// Check if we already have a shard assigned for this context
	if val, ok := w.ctxShards.Load(ctx); ok {
		idx := val.(int)
		return shards[idx], true, false
	}

	// Use hash as starting point to avoid always checking from index 0
	type iface struct {
		typ  uintptr
		data uintptr
	}
	ptr := (*iface)(unsafe.Pointer(&ctx)).data
	idx1 := int(ptr % uintptr(n))
	idx2 := int((ptr >> 8) % uintptr(n))
	selected := idx1
	load1 := shards[idx1].load.Load()
	load2 := shards[idx2].load.Load()
	finalLoad := load1
	if shards[idx2].load.Load() < shards[idx1].load.Load() {
		selected = idx2
		finalLoad = load2
	}
	if finalLoad > maxCasesPerShard {
		// fallback
		return nil, false, true
	}

	actual, loaded := w.ctxShards.LoadOrStore(ctx, selected)
	if loaded {
		idx := actual.(int)
		return shards[idx], true, false
	}

	w.activeContexts.Add(1)
	return shards[selected], false, false
}

func (w *contextWatcherReflect) removeActiveContext(ctx context.Context) {
	w.ctxShards.Delete(ctx)
	w.activeContexts.Add(-1)
}

func (w *contextWatcherReflect) RegisterContext(ctx context.Context, callback func(context.Context)) {
	if ctx.Done() == nil {
		return
	}
	select {
	case <-ctx.Done():
		gofunc.GoFunc(ctx, func() {
			callback(ctx)
		})
		return
	default:
	}

	shard, exist, fallback := w.getShard(ctx)
	if fallback {
		gofunc.GoFunc(ctx, func() {
			callback(ctx)
		})
		return
	}
	if exist {
		return
	}

	shard.registerContext(ctx, callback)
	// Check if we need to expand shards
	w.checkAndExpand()
}

func (w *contextWatcherReflect) DeregisterContext(ctx context.Context) {
	// Only deregister if we have a record of this context
	// This prevents sending delete to wrong shard if ctxShards was already cleaned
	val, ok := w.ctxShards.Load(ctx)
	if !ok {
		// Context not registered or already cleaned up
		return
	}

	idx := val.(int)
	shards := *w.shards.Load()

	shards[idx].deregisterContext(ctx)
}

// checkAndExpand checks if shard expansion is needed based on current load
// and triggers expansion if necessary. This is called automatically after
// each RegisterContext call.
func (w *contextWatcherReflect) checkAndExpand() {
	activeCount := w.activeContexts.Load()
	shards := *w.shards.Load()
	shardsNum := len(shards)

	// Don't expand if:
	// 1. Already at maximum shard count
	// 2. Total contexts below minimum threshold
	// 3. Average load per shard is acceptable
	if activeCount < minContextsForExpand {
		return
	}
	if shardsNum >= maxShardsNum {
		return
	}
	avgLoad := activeCount / int64(shardsNum)
	if avgLoad <= shardLoadThreshold {
		return
	}

	// Trigger expansion asynchronously to avoid blocking registration
	if w.expandMutex.TryLock() {
		go func() {
			w.expandShardsLocked()
			w.expandMutex.Unlock()
		}()
	}
}

// expandShardsLocked is the internal expansion implementation
// Caller must hold expandMutex
func (w *contextWatcherReflect) expandShardsLocked() {
	oldShards := *w.shards.Load()
	oldNum := len(oldShards)
	newNum := oldNum * 2
	if newNum > maxShardsNum {
		newNum = maxShardsNum
	}
	if newNum == oldNum {
		return
	}

	newShards := make([]*reflectShard, newNum)

	// existing contexts stay in their original shards
	copy(newShards, oldShards)

	for i := oldNum; i < newNum; i++ {
		newShards[i] = newReflectShard(w, w.shardCapacity, w.signalCapacity)
		go newShards[i].run()
	}

	w.shards.Store(&newShards)
}

const (
	defaultShardNum      = 128
	shardLoadThreshold   = 64  // Trigger expansion when average load per shard exceeds this (reduced from 256)
	minContextsForExpand = 512 // Minimum total contexts before considering expansion (reduced from 1024)
	maxCasesPerShard     = 128 // Hard limit: max cases per shard to maintain reflect.Select performance
	maxShardsNum         = 8192
)

type operationType uint8

const (
	createType operationType = iota + 1
	deleteType
)

type reflectItem struct {
	opType   operationType
	ctx      context.Context
	callback func(context.Context)
}

// reflectShard monitors multiple contexts using a single goroutine with reflect.Select.
// The cases slice contains SelectCases: cases[0] is the signal channel for control messages,
// and cases[1..n] are context.Done() channels. The items slice stores corresponding metadata.
type reflectShard struct {
	watcher    *contextWatcherReflect
	cases      []reflect.SelectCase
	items      []reflectItem
	ctxIndexes map[context.Context]int
	signal     chan reflectItem
	load       atomic.Int32 // current number of active contexts in this shard
}

func newReflectShard(watcher *contextWatcherReflect, shardCapacity, signalCapacity int) *reflectShard {
	signal := make(chan reflectItem, signalCapacity)
	cases := make([]reflect.SelectCase, 1, shardCapacity+1)
	cases[0] = reflect.SelectCase{
		Dir:  reflect.SelectRecv,
		Chan: reflect.ValueOf(signal),
	}
	items := make([]reflectItem, 1, shardCapacity+1)
	return &reflectShard{
		watcher:    watcher,
		cases:      cases,
		items:      items,
		ctxIndexes: make(map[context.Context]int),
		signal:     signal,
	}
}

func (s *reflectShard) registerContext(ctx context.Context, callback func(context.Context)) {
	s.signal <- reflectItem{
		opType:   createType,
		ctx:      ctx,
		callback: callback,
	}
}

func (s *reflectShard) deregisterContext(ctx context.Context) {
	s.signal <- reflectItem{
		opType: deleteType,
		ctx:    ctx,
	}
}

func (s *reflectShard) run() {
	for {
		chosen, recvVal, _ := reflect.Select(s.cases)
		if chosen == 0 {
			// signal
			item := recvVal.Interface().(reflectItem)
			switch item.opType {
			case createType:
				// Check for duplicate registration
				if _, ok := s.ctxIndexes[item.ctx]; ok {
					continue
				}

				// Check if context is already done - no need to register
				select {
				case <-item.ctx.Done():
					// Context already cancelled, execute callback immediately
					gofunc.GoFunc(item.ctx, func() {
						item.callback(item.ctx)
					})
					s.watcher.removeActiveContext(item.ctx)
					continue
				default:
				}

				s.cases = append(s.cases, reflect.SelectCase{
					Dir:  reflect.SelectRecv,
					Chan: reflect.ValueOf(item.ctx.Done()),
				})
				s.items = append(s.items, item)
				s.ctxIndexes[item.ctx] = len(s.cases) - 1
				s.load.Add(1) // Increment shard load

			case deleteType:
				idx, ok := s.ctxIndexes[item.ctx]
				if !ok {
					continue
				}
				s.removeItemAtIndex(idx)
				delete(s.ctxIndexes, item.ctx)
				s.load.Add(-1) // Decrement shard load
				s.watcher.removeActiveContext(item.ctx)
			}
			continue
		}
		item := s.items[chosen]
		gofunc.GoFunc(item.ctx, func() {
			item.callback(item.ctx)
		})
		s.removeItemAtIndex(chosen)
		delete(s.ctxIndexes, item.ctx)
		s.load.Add(-1) // Decrement shard load when context is done
		s.watcher.removeActiveContext(item.ctx)
	}
}

func (s *reflectShard) removeItemAtIndex(idx int) {
	lastIdx := len(s.cases) - 1
	if idx != lastIdx {
		s.cases[idx] = s.cases[lastIdx]
		s.items[idx] = s.items[lastIdx]

		moved := s.items[idx].ctx
		s.ctxIndexes[moved] = idx
	}
	s.cases[lastIdx] = reflect.SelectCase{}
	s.items[lastIdx] = reflectItem{}

	s.cases = s.cases[:lastIdx]
	s.items = s.items[:lastIdx]
}
