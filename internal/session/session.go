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
	"time"

	"github.com/cloudwego/localsession"
)

var sessionEnabled bool

// Options
type Options struct {
	Enable bool
	localsession.ManagerOptions
}

// Default Options
func NewManagerOptions() localsession.ManagerOptions {
	return localsession.ManagerOptions{
		EnableImplicitlyTransmitAsync: false,
		ShardNumber:                   100,
		GCInterval:                    time.Hour,
	}
}

// Init session Manager
// It uses env config first, the key is localsession.SESSION_CONFIG_KEY
//
//go:nocheckptr
func Init(opts Options) {
	if opts.Enable {
		localsession.ResetDefaultManager(opts.ManagerOptions)
		sessionEnabled = true
	} else if sessionEnabled {
		sessionEnabled = false
	}
}

// Get current session
func CurSession() (localsession.Session, bool) {
	if !sessionEnabled {
		return nil, false
	}
	return localsession.CurSession()
}

// Set current Sessioin
func BindSession(ctx context.Context) {
	if !sessionEnabled {
		return
	}
	localsession.BindSession(localsession.NewSessionCtx(ctx))
}

// Unset current Session
func UnbindSession() {
	if !sessionEnabled {
		return
	}
	localsession.UnbindSession()
}
