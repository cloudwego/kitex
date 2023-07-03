/*
 * Copyright 2021 CloudWeGo Authors
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

package rpcinfo

import (
	"context"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/cloudwego/kitex/internal"

	"github.com/bytedance/gopkg/util/session"
)

const KITEX_SESSION_CONFIG_KEY = "KITEX_SESSION_CONFIG"

func init() {
	opts := strings.Split(os.Getenv(KITEX_SESSION_CONFIG_KEY), ",")
	if len(opts) == 0 {
		return
	}
	shards := 100
	// parse first option as ShardNumber
	if opt, err := strconv.Atoi(opts[0]); err == nil {
		shards = opt
	}
	async := false
	// parse second option as EnableTransparentTransmitAsync
	if len(opts) > 1 && strings.ToLower(opts[1]) == "async" {
		async = true
	}
	GCInterval := time.Hour
	// parse third option as EnableTransparentTransmitAsync
	if len(opts) > 2 {
		if d, err := time.ParseDuration(opts[2]); err == nil && d > time.Second {
			GCInterval = d
		}
	}
	session.SetDefaultManager(session.NewSessionManager(session.ManagerOptions{
		EnableTransparentTransmitAsync: async,
		ShardNumber:                    shards,
		GCInterval:                     GCInterval,
	}))
}

type ctxRPCInfoKeyType struct{}

var ctxRPCInfoKey ctxRPCInfoKeyType

// NewCtxWithRPCInfo creates a new context with the RPCInfo given.
func NewCtxWithRPCInfo(ctx context.Context, ri RPCInfo) context.Context {
	if ri != nil {
		ctx := context.WithValue(ctx, ctxRPCInfoKey, ri)
		session.BindSession(session.NewSessionCtx(ctx))
		return ctx
	}
	return ctx
}

// GetRPCInfo gets RPCInfo from ctx.
// Returns nil if not found.
func GetRPCInfo(ctx context.Context) RPCInfo {
	if ri, ok := ctx.Value(ctxRPCInfoKey).(RPCInfo); ok {
		return ri
	}
	s, ok := session.CurSession()
	if ok && s.IsValid() {
		if ri, ok := s.Get(ctxRPCInfoKey).(RPCInfo); ok {
			return ri
		}
	}
	return nil
}

// PutRPCInfo recycles the RPCInfo. This function is for internal use only.
func PutRPCInfo(ri RPCInfo) {
	session.UnbindSession()
	if v, ok := ri.(internal.Reusable); ok {
		v.Recycle()
	}
}

// FreezeRPCInfo returns a new context containing an RPCInfo that is safe
// to be used asynchronically.
// Note that the RPCStats of the freezed RPCInfo will be nil and
// the FreezeRPCInfo itself should not be used asynchronically.
//
// Example:
//
//	func (p *MyServiceImpl) MyMethod(ctx context.Context, req *MyRequest) (resp *MyResponse, err error) {
//	    ri := rpcinfo.GetRPCInfo(ctx)
//	    go func(ctx context.Context) {
//	        ...
//	        ri := rpcinfo.GetRPCInfo(ctx) // not concurrent-safe
//	        ...
//	    }(ctx2)
//
//	    ctx2 := rpcinfo.FreezeRPCInfo(ctx) // this creates a read-only copy of `ri` and attaches it to the new context
//	    go func(ctx context.Context) {
//	        ...
//	        ri := rpcinfo.GetRPCInfo(ctx) // OK
//	        ...
//	    }(ctx2)
//	}
func FreezeRPCInfo(ctx context.Context) context.Context {
	ri := GetRPCInfo(ctx)
	if ri == nil {
		return ctx
	}
	return NewCtxWithRPCInfo(ctx, freeze(ri))
}
