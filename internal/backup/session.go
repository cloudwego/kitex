/**
 * Copyright 2023 CloudWeGo Authors
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

package backup

import (
	"context"
	"sync"

	"github.com/bytedance/gopkg/cloud/metainfo"
	"github.com/cloudwego/localsession"
)

var (
	shouldUseSession = func(ctx context.Context) bool {
		return !metainfo.HasMetaInfo(ctx)
	}
	initOnce sync.Once
)

// Options
type Options struct {
	Enable           bool
	ShouldUseSession func(ctx context.Context) bool
	localsession.ManagerOptions
}

// Default Options
func NewManagerOptions() localsession.ManagerOptions {
	return localsession.DefaultManagerOptions()
}

// Init session Manager
// It uses env config first, the key is localsession.SESSION_CONFIG_KEY
func Init(opts Options) {
	if opts.Enable {
		initOnce.Do(func() {
			localsession.InitDefaultManager(opts.ManagerOptions)
			if opts.ShouldUseSession != nil {
				shouldUseSession = opts.ShouldUseSession
			}
		})
	}
}

// Get current ctx
func CurCtx(ctx context.Context) context.Context {
	if !shouldUseSession(ctx) {
		return ctx
	}
	s, ok := localsession.CurSession()
	if !ok {
		return ctx
	}
	c, ok := s.(localsession.SessionCtx)
	if !ok {
		return ctx
	}
	c1 := c.Export()

	// two-way merge all persistent kvs
	if n := metainfo.CountPersistentValues(c1); n > 0 {
		// persistent kvs
		kvs := make([]string, 0, n*2)
		mkvs := metainfo.GetAllPersistentValues(ctx)

		// incoming ctx is prior to session
		if mkvs == nil {
			metainfo.RangePersistentValues(c1, func(k, v string) bool {
				kvs = append(kvs, k, v)
				return true
			})
		} else {
			kvs = make([]string, 0, n*2)
			metainfo.RangePersistentValues(c1, func(k, v string) bool {
				if _, ok := mkvs[k]; !ok {
					kvs = append(kvs, k, v)
				}
				return true
			})
		}
		ctx = metainfo.WithPersistentValues(ctx, kvs...)
	}
	return ctx
}

// Set current Sessioin
func BackupCtx(ctx context.Context) {
	localsession.BindSession(localsession.NewSessionCtx(ctx))
}

// Unset current Session
func ClearCtx() {
	localsession.UnbindSession()
}
