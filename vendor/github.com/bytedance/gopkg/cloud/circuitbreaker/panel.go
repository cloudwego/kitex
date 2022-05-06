// Copyright 2021 ByteDance Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package circuitbreaker

import (
	"sync"
	"time"

	"github.com/bytedance/gopkg/collection/skipmap"
)

// panel manages a batch of circuitbreakers
type panel struct {
	breakers       *skipmap.StringMap
	defaultOptions Options
	changeHandler  PanelStateChangeHandler
}

type sharedTicker struct {
	sync.Mutex
	started  bool
	stopChan chan bool
	ticker   *time.Ticker
	panels   map[*panel]struct{}
}

var tickerMap sync.Map // 共用 ticker

// NewPanel .
func NewPanel(changeHandler PanelStateChangeHandler,
	defaultOptions Options) (Panel, error) {
	if defaultOptions.BucketTime <= 0 {
		defaultOptions.BucketTime = defaultBucketTime
	}

	if defaultOptions.BucketNums <= 0 {
		defaultOptions.BucketNums = defaultBucketNums
	}

	if defaultOptions.CoolingTimeout <= 0 {
		defaultOptions.CoolingTimeout = defaultCoolingTimeout
	}

	if defaultOptions.DetectTimeout <= 0 {
		defaultOptions.DetectTimeout = defaultDetectTimeout
	}
	_, err := newBreaker(defaultOptions)
	if err != nil {
		return nil, err
	}
	p := &panel{
		breakers:       skipmap.NewString(),
		defaultOptions: defaultOptions,
		changeHandler:  changeHandler,
	}
	ti, _ := tickerMap.LoadOrStore(p.defaultOptions.BucketTime,
		&sharedTicker{panels: make(map[*panel]struct{}), stopChan: make(chan bool, 1)})
	t := ti.(*sharedTicker)
	t.Lock()
	t.panels[p] = struct{}{}
	if !t.started {
		t.started = true
		t.ticker = time.NewTicker(p.defaultOptions.BucketTime)
		go t.tick(t.ticker)
	}
	t.Unlock()
	return p, nil
}

// getBreaker .
func (p *panel) getBreaker(key string) *breaker {
	cb, ok := p.breakers.Load(key)
	if ok {
		return cb.(*breaker)
	}

	op := p.defaultOptions
	if p.changeHandler != nil {
		op.BreakerStateChangeHandler = func(oldState, newState State, m Metricer) {
			p.changeHandler(key, oldState, newState, m)
		}
	}
	ncb, _ := newBreaker(op)
	cb, ok = p.breakers.LoadOrStore(key, ncb)
	return cb.(*breaker)
}

// RemoveBreaker .
func (p *panel) RemoveBreaker(key string) {
	p.breakers.Delete(key)
}

// DumpBreakers .
func (p *panel) DumpBreakers() map[string]Breaker {
	breakers := make(map[string]Breaker)
	p.breakers.Range(func(key string, value interface{}) bool {
		breakers[key] = value.(*breaker)
		return true
	})
	return breakers
}

// Succeed .
func (p *panel) Succeed(key string) {
	p.getBreaker(key).Succeed()
}

// Fail .
func (p *panel) Fail(key string) {
	b := p.getBreaker(key)
	if p.defaultOptions.ShouldTripWithKey != nil {
		b.FailWithTrip(p.defaultOptions.ShouldTripWithKey(key))
	} else {
		b.Fail()
	}
}

// FailWithTrip .
func (p *panel) FailWithTrip(key string, f TripFunc) {
	p.getBreaker(key).FailWithTrip(f)
}

// Timeout .
func (p *panel) Timeout(key string) {
	b := p.getBreaker(key)
	if p.defaultOptions.ShouldTripWithKey != nil {
		b.TimeoutWithTrip(p.defaultOptions.ShouldTripWithKey(key))
	} else {
		b.Timeout()
	}
}

// TimeoutWithTrip .
func (p *panel) TimeoutWithTrip(key string, f TripFunc) {
	p.getBreaker(key).TimeoutWithTrip(f)
}

// IsAllowed .
func (p *panel) IsAllowed(key string) bool {
	return p.getBreaker(key).IsAllowed()
}

// GetMetricer ...
func (p *panel) GetMetricer(key string) Metricer {
	return p.getBreaker(key).Metricer()
}

func (p *panel) Close() {
	ti, _ := tickerMap.Load(p.defaultOptions.BucketTime)
	t := ti.(*sharedTicker)
	t.Lock()
	delete(t.panels, p)
	if len(t.panels) == 0 {
		t.stopChan <- true
		t.started = false
	}
	t.Unlock()
}

// tick .
// pass ticker but not use t.ticker directly is to ignore race.
func (t *sharedTicker) tick(ticker *time.Ticker) {
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			t.Lock()
			for p := range t.panels {
				p.breakers.Range(func(_ string, value interface{}) bool {
					if b, ok := value.(*breaker); ok {
						b.metricer.tick()
					}
					return true
				})
			}
			t.Unlock()
		case stop := <-t.stopChan:
			if stop {
				return
			}
		}
	}
}
