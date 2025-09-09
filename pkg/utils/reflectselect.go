package utils

import (
	"reflect"
	"sync"

	"github.com/bytedance/gopkg/util/gopool"
)

const (
	// Maximum number of cases each selector can handle
	singleSelectorMaxCases = 127
)

// singleSelector handles a batch of channels (up to singleSelectorMaxCases)
type singleSelector struct {
	*CtxDoneSelector
	cases      []reflect.SelectCase
	signalChan chan struct{}
}

func (ds *CtxDoneSelector) newSingleSelector() *singleSelector {
	ss := &singleSelector{
		cases:           make([]reflect.SelectCase, 0, singleSelectorMaxCases),
		signalChan:      make(chan struct{}, 1),
		CtxDoneSelector: ds,
	}
	go ss.run()
	return ss
}

func (ss *singleSelector) run() {
	var hasRun bool
	cases := make([]reflect.SelectCase, singleSelectorMaxCases+1)
	for {
		ss.mu.Lock()
		if ss.stop == 1 || (len(ss.cases) == 0 && hasRun) {
			ss.mu.Unlock()
			return
		}
		hasRun = true
		n := copy(cases, ss.cases)
		cases[n] = reflect.SelectCase{
			Dir:  reflect.SelectRecv,
			Chan: reflect.ValueOf(ss.signalChan),
		}
		ss.mu.Unlock()
		chosen, _, _ := reflect.Select(cases[:n+1])
		if chosen != n {
			c := cases[chosen].Chan.Interface().(<-chan struct{})
			ss.mu.Lock()
			// run
			if val, exist := ss.chMap[c]; exist {
				gopool.Go(val.callback)
			}
			// delete
			ss.delete(c)
			ss.mu.Unlock()
		}
	}
}

func (ss *singleSelector) signal() {
	select {
	case ss.signalChan <- struct{}{}:
	default:
	}
}

type chMapVal struct {
	index    uint32
	callback func()
}

type CtxDoneSelector struct {
	mu        sync.Mutex
	chMap     map[<-chan struct{}]chMapVal
	count     uint32
	selectors []*singleSelector
	stop      uint32 // 0: ok, 1: closed
}

// NewCtxDoneSelector creates a new selector instance.
func NewCtxDoneSelector() *CtxDoneSelector {
	return &CtxDoneSelector{
		chMap: make(map[<-chan struct{}]chMapVal),
	}
}

// Add a done channel with callback.
// Parameters:
//
//	done: channel that will be closed when done (typically ctx.Done())
//	cb:   callback function to execute when channel closes
//
// Note: Channels must be comparable. Identical channels will be treated as same.
func (ds *CtxDoneSelector) Add(done <-chan struct{}, cb func()) {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	if ds.stop == 1 {
		return
	}

	if _, exist := ds.chMap[done]; exist {
		return
	}

	count := ds.count
	ds.count++
	if count%singleSelectorMaxCases == 0 {
		// create a new selector
		ds.selectors = append(ds.selectors, ds.newSingleSelector())
	}
	selector := ds.selectors[count/singleSelectorMaxCases]
	selector.cases = append(selector.cases, reflect.SelectCase{
		Dir:  reflect.SelectRecv,
		Chan: reflect.ValueOf(done),
	})
	selector.signal()
	ds.chMap[done] = chMapVal{index: count, callback: cb}
}

// Delete a channel before it triggers.
func (ds *CtxDoneSelector) Delete(done <-chan struct{}) {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	if ds.stop == 1 {
		return
	}

	ds.delete(done)
}

func (ds *CtxDoneSelector) delete(done <-chan struct{}) {
	val, exist := ds.chMap[done]
	if !exist {
		return
	}
	curSelector := ds.selectors[val.index/singleSelectorMaxCases]
	lastSelector := ds.selectors[len(ds.selectors)-1]
	// move last case to the deleted position
	lastCase := lastSelector.cases[len(lastSelector.cases)-1]
	curSelector.cases[val.index%singleSelectorMaxCases] = lastCase
	// update last case's index
	lastChan := lastCase.Chan.Interface().(<-chan struct{})
	ds.chMap[lastChan] = chMapVal{callback: ds.chMap[lastChan].callback, index: val.index}
	// delete
	ds.count--
	delete(ds.chMap, done)
	lastSelector.cases = lastSelector.cases[:len(lastSelector.cases)-1]
	if len(lastSelector.cases) == 0 {
		ds.selectors = ds.selectors[:len(ds.selectors)-1]
	}
	// signal selectors
	curSelector.signal()
	lastSelector.signal()
}

// Close gracefully shuts down the selector.
// After close:
//   - New Add/Delete calls are ignored.
//   - Pending callbacks may still execute.
//   - Resources are released.
func (ds *CtxDoneSelector) Close() {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	ds.stop = 1
	for _, selector := range ds.selectors {
		selector.signal()
	}
}
