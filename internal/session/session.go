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
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	gs "github.com/cloudwego/localsession"
)

// Session Env config key
const KITEX_SESSION_CONFIG_KEY = "KITEX_SESSION_CONFIG"

var (
	sessionEnabled bool
	sessionOnce    sync.Once
)

// Options
type Options struct {
	Enable bool
	gs.ManagerOptions
}

// Default Options
func NewManagerOptions() gs.ManagerOptions {
	return gs.ManagerOptions{
		EnableImplicitlyTransmitAsync: false,
		ShardNumber:                   100,
		GCInterval:                    time.Hour,
	}
}

// Init session Manager
// It uses env config first, the key is KITEX_SESSION_CONFIG_KEY
//
//go:nocheckptr
func Init(opts *Options) {
	// check env first
	if env := os.Getenv(KITEX_SESSION_CONFIG_KEY); env != "" {
		envs := strings.Split(env, ",")
		op := gs.ManagerOptions{}
		op.ShardNumber = gs.DefaultShardNum
		// parse first option as ShardNumber
		if opt, err := strconv.Atoi(envs[0]); err == nil {
			op.ShardNumber = opt
		}
		op.EnableImplicitlyTransmitAsync = false
		// parse second option as EnableTransparentTransmitAsync
		if len(envs) > 1 && strings.ToLower(envs[1]) == "async" {
			op.EnableImplicitlyTransmitAsync = true
		}
		op.GCInterval = gs.DefaultGCInterval
		// parse third option as EnableTransparentTransmitAsync
		if len(envs) > 2 {
			if d, err := time.ParseDuration(envs[2]); err == nil && d > time.Second {
				op.GCInterval = d
			}
		}
		opts = &Options{
			Enable:         true,
			ManagerOptions: op,
		}
		// no env found, then check argument
	} else if opts == nil {
		return
	}

	if opts.Enable {
		sessionOnce.Do(func() {
			gs.SetDefaultManager(gs.NewSessionManager(opts.ManagerOptions))
		})
		sessionEnabled = true
	} else {
		sessionEnabled = false
	}
}

// Get current session
func CurSession() (gs.Session, bool) {
	if !sessionEnabled {
		return nil, false
	}
	return gs.CurSession()
}

// Set current Sessioin
func BindSession(ctx context.Context) {
	if !sessionEnabled {
		return
	}
	gs.BindSession(gs.NewSessionCtx(ctx))
}

// Unset current Session
func UnbindSession() {
	if !sessionEnabled {
		return
	}
	gs.UnbindSession()
}
