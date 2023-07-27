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

package session

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
//
//go:nocheckptr
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

// Get current session
func CurSession(ctx context.Context) context.Context {
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
	c2 := c.Export()

	// two-way merge all persistent kvs, incoming ctx is prior to session
	if metainfo.HasMetaInfo(ctx) {
		kvs := make([]string, 0, 16) // opt: get kvs size from metainfo API
		metainfo.RangePersistentValues(ctx, func(k, v string) bool {
			kvs = append(kvs, k, v)
			return true
		})
		c2 = metainfo.WithPersistentValues(c2, kvs...)
	}
	return c2
}

// Set current Sessioin
func BindSession(ctx context.Context) {
	localsession.BindSession(localsession.NewSessionCtx(ctx))
}

// Unset current Session
func UnbindSession() {
	localsession.UnbindSession()
}
