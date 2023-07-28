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
	initOnce sync.Once

	shouldUseSession func(ctx context.Context) bool

	backupHandler func(pre, cur context.Context) context.Context
)

// Options
type Options struct {
	Enable           bool
	ShouldUseSession func(ctx context.Context) bool
	BackupHandler    func(prev, cur context.Context) context.Context
	localsession.ManagerOptions
}

// Default Options
func DefaultOptions() Options {
	return Options{
		Enable: false,
		ShouldUseSession: func(ctx context.Context) bool {
			return !metainfo.HasMetaInfo(ctx)
		},
		BackupHandler:  nil,
		ManagerOptions: localsession.DefaultManagerOptions(),
	}
}

// Init session Manager
// It uses env config first, the key is localsession.SESSION_CONFIG_KEY
func Init(opts Options) {
	if opts.Enable {
		initOnce.Do(func() {
			localsession.InitDefaultManager(opts.ManagerOptions)
			shouldUseSession = opts.ShouldUseSession
			backupHandler = opts.BackupHandler
		})
	}
}

// If Options.ShouldUseSession() == true,
// this func will try to merge metainfo
// and pre-defined key-values (through Options.BackupHanlder)
// from backup context into given context
func RecoverCtxOndemands(ctx context.Context) context.Context {
	if shouldUseSession == nil || !shouldUseSession(ctx) {
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
	pre := c.Export()

	// two-way merge all persistent metainfo if pre context has
	if n := metainfo.CountPersistentValues(pre); n > 0 {
		// persistent kvs
		kvs := make([]string, 0, n*2)
		mkvs := metainfo.GetAllPersistentValues(ctx)

		// incoming ctx is prior to session
		if len(mkvs) == 0 {
			// merge all kvs from pre
			metainfo.RangePersistentValues(pre, func(k, v string) bool {
				kvs = append(kvs, k, v)
				return true
			})
		} else {
			kvs = make([]string, 0, n*2)
			metainfo.RangePersistentValues(pre, func(k, v string) bool {
				// filter kvs which exists in cur
				if _, ok := mkvs[k]; !ok {
					kvs = append(kvs, k, v)
				}
				return true
			})
		}
		ctx = metainfo.WithPersistentValues(ctx, kvs...)
	}

	// trigger user-defined handler if any
	if backupHandler != nil {
		ctx = backupHandler(pre, ctx)
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
