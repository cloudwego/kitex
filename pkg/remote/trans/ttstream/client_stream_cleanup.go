/*
 * Copyright 2025 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package ttstream

import (
	"sync"
	"time"

	"github.com/cloudwego/kitex/pkg/streaming"
	"github.com/cloudwego/kitex/pkg/utils"
)

const defaultStreamCleanupInterval time.Duration = 5 * time.Second

var defaultStreamCleanupConfig = streaming.StreamCleanupConfig{
	Enable:        true,
	CleanInterval: defaultStreamCleanupInterval,
}

var globalTickerManager = newTickerManager()

type tickerManager struct {
	tickers sync.Map // key: time.Duration, val: *utils.SharedTicker
}

func newTickerManager() *tickerManager {
	tm := &tickerManager{}
	tm.tickers.Store(defaultStreamCleanupInterval, utils.NewSyncSharedTicker(defaultStreamCleanupInterval))
	return tm
}

func (tm *tickerManager) getOrSetNewTicker(interval time.Duration) *utils.SharedTicker {
	tVal, ok := tm.tickers.Load(interval)
	if ok {
		return tVal.(*utils.SharedTicker)
	}
	tVal, _ = tm.tickers.LoadOrStore(interval, utils.NewSyncSharedTicker(interval))
	return tVal.(*utils.SharedTicker)
}

func (t *transport) Tick() {
	var toCloseStreams []*stream
	t.streams.Range(func(key, value any) bool {
		s := value.(*stream)
		select {
		case <-s.ctx.Done():
			toCloseStreams = append(toCloseStreams, s)
		default:
		}
		return true
	})
	for _, s := range toCloseStreams {
		s.ctxDoneCallback(s.ctx)
	}
}
