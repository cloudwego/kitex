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

package metainfo

import (
	"context"
	"sync"
)

type bwCtxKeyType int

const (
	bwCtxKeySend bwCtxKeyType = iota
	bwCtxKeyRecv
)

type bwCtxValue struct {
	sync.RWMutex
	kvs map[string]string
}

func newBackwardCtxValues() *bwCtxValue {
	return &bwCtxValue{
		kvs: make(map[string]string),
	}
}

func (p *bwCtxValue) get(k string) (v string, ok bool) {
	p.RLock()
	v, ok = p.kvs[k]
	p.RUnlock()
	return
}

func (p *bwCtxValue) getAll() (m map[string]string) {
	p.RLock()
	if cnt := len(p.kvs); cnt > 0 {
		m = make(map[string]string, cnt)
		for k, v := range p.kvs {
			m[k] = v
		}
	}
	p.RUnlock()
	return
}

func (p *bwCtxValue) set(k, v string) {
	p.Lock()
	p.kvs[k] = v
	p.Unlock()
}

func (p *bwCtxValue) setMany(kvs []string) {
	p.Lock()
	for i := 0; i < len(kvs); i += 2 {
		p.kvs[kvs[i]] = kvs[i+1]
	}
	p.Unlock()
}

func (p *bwCtxValue) setMap(kvs map[string]string) {
	p.Lock()
	for k, v := range kvs {
		p.kvs[k] = v
	}
	p.Unlock()
}

// WithBackwardValues returns a new context that allows passing key-value pairs
// backward with `SetBackwardValue` from any derived context.
func WithBackwardValues(ctx context.Context) context.Context {
	if _, ok := ctx.Value(bwCtxKeyRecv).(*bwCtxValue); ok {
		return ctx
	}
	ctx = context.WithValue(ctx, bwCtxKeyRecv, newBackwardCtxValues())
	return ctx
}

// RecvBackwardValue gets a value associated with the given key that is set by
// `SetBackwardValue` or `SetBackwardValues`.
func RecvBackwardValue(ctx context.Context, key string) (val string, ok bool) {
	if p, exist := ctx.Value(bwCtxKeyRecv).(*bwCtxValue); exist {
		val, ok = p.get(key)
	}
	return
}

// RecvAllBackwardValues is the batched version of RecvBackwardValue.
func RecvAllBackwardValues(ctx context.Context) (m map[string]string) {
	if p, ok := ctx.Value(bwCtxKeyRecv).(*bwCtxValue); ok {
		return p.getAll()
	}
	return
}

// SetBackwardValue sets a key value pair into the context.
func SetBackwardValue(ctx context.Context, key, val string) (ok bool) {
	if p, ok := ctx.Value(bwCtxKeyRecv).(*bwCtxValue); ok {
		p.set(key, val)
		return true
	}
	return false
}

// SetBackwardValues is the batched version of `SetBackwardValue`.
func SetBackwardValues(ctx context.Context, kvs ...string) (ok bool) {
	if l := len(kvs); l > 0 && l%2 == 0 {
		if p, ok := ctx.Value(bwCtxKeyRecv).(*bwCtxValue); ok {
			p.setMany(kvs)
			return true
		}
	}
	return
}

// SetBackwardValuesFromMap is the batched version of `SetBackwardValue`.
func SetBackwardValuesFromMap(ctx context.Context, kvs map[string]string) (ok bool) {
	if l := len(kvs); l > 0 {
		if p, ok := ctx.Value(bwCtxKeyRecv).(*bwCtxValue); ok {
			p.setMap(kvs)
			return true
		}
	}
	return
}

// WithBackwardValuesToSend returns a new context that collects key-value
// pairs set with `SendBackwardValue` or `SendBackwardValues` into any
// derived context.
func WithBackwardValuesToSend(ctx context.Context) context.Context {
	if _, ok := ctx.Value(bwCtxKeySend).(*bwCtxValue); ok {
		return ctx
	}
	ctx = context.WithValue(ctx, bwCtxKeySend, newBackwardCtxValues())
	return ctx
}

// SendBackwardValue sets a key-value pair into the context for sending to
// a remote endpoint.
// Note that the values can not be retrieved with `RecvBackwardValue` from
// the same context.
func SendBackwardValue(ctx context.Context, key, val string) (ok bool) {
	if p, ok := ctx.Value(bwCtxKeySend).(*bwCtxValue); ok {
		p.set(key, val)
		return true
	}
	return
}

// SendBackwardValues is the batched version of `SendBackwardValue`.
func SendBackwardValues(ctx context.Context, kvs ...string) (ok bool) {
	if l := len(kvs); l > 0 && l%2 == 0 {
		if p, ok := ctx.Value(bwCtxKeySend).(*bwCtxValue); ok {
			p.setMany(kvs)
			return true
		}
	}
	return
}

// SendBackwardValuesFromMap is the batched version of `SendBackwardValue`.
func SendBackwardValuesFromMap(ctx context.Context, kvs map[string]string) (ok bool) {
	if l := len(kvs); l > 0 {
		if p, ok := ctx.Value(bwCtxKeySend).(*bwCtxValue); ok {
			p.setMap(kvs)
			return true
		}
	}
	return
}

// AllBackwardValuesToSend retrieves all key-values pairs set by `SendBackwardValue`
// or `SendBackwardValues` from the given context.
// This function is designed for frameworks, common developers should not use it.
func AllBackwardValuesToSend(ctx context.Context) (m map[string]string) {
	if p, ok := ctx.Value(bwCtxKeySend).(*bwCtxValue); ok {
		return p.getAll()
	}
	return
}

// GetBackwardValue is an alias to `RecvBackwardValue`.
// Deprecated: for clarity, please use `RecvBackwardValue` instead.
func GetBackwardValue(ctx context.Context, key string) (val string, ok bool) {
	return RecvBackwardValue(ctx, key)
}

// GetAllBackwardValues is an alias to RecvAllBackwardValues.
// Deprecated: for clarity, please use `RecvAllBackwardValues` instead.
func GetAllBackwardValues(ctx context.Context) map[string]string {
	return RecvAllBackwardValues(ctx)
}
