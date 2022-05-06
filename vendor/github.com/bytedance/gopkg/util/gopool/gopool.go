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

package gopool

import (
	"context"
	"fmt"
	"sync"
)

// defaultPool is the global default pool.
var defaultPool Pool

var poolMap sync.Map

func init() {
	defaultPool = NewPool("gopool.DefaultPool", 10000, NewConfig())
}

// Go is an alternative to the go keyword, which is able to recover panic.
// gopool.Go(func(arg interface{}){
//     ...
// }(nil))
func Go(f func()) {
	CtxGo(context.Background(), f)
}

// CtxGo is preferred than Go.
func CtxGo(ctx context.Context, f func()) {
	defaultPool.CtxGo(ctx, f)
}

// SetCap is not recommended to be called, this func changes the global pool's capacity which will affect other callers.
func SetCap(cap int32) {
	defaultPool.SetCap(cap)
}

// SetPanicHandler sets the panic handler for the global pool.
func SetPanicHandler(f func(context.Context, interface{})) {
	defaultPool.SetPanicHandler(f)
}

// RegisterPool registers a new pool to the global map.
// GetPool can be used to get the registered pool by name.
// returns error if the same name is registered.
func RegisterPool(p Pool) error {
	_, loaded := poolMap.LoadOrStore(p.Name(), p)
	if loaded {
		return fmt.Errorf("name: %s already registered", p.Name())
	}
	return nil
}

// GetPool gets the registered pool by name.
// Returns nil if not registered.
func GetPool(name string) Pool {
	p, ok := poolMap.Load(name)
	if !ok {
		return nil
	}
	return p.(Pool)
}
