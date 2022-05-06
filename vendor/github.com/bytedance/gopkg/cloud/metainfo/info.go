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
)

// The prefix listed below may be used to tag the types of values when there is no context to carry them.
const (
	PrefixPersistent         = "RPC_PERSIST_"
	PrefixTransient          = "RPC_TRANSIT_"
	PrefixTransientUpstream  = "RPC_TRANSIT_UPSTREAM_"
	PrefixBackward           = "RPC_BACKWARD_"
	PrefixBackwardDownstream = "RPC_BACKWARD_DOWNSTREAM_"

	lenPTU = len(PrefixTransientUpstream)
	lenPT  = len(PrefixTransient)
	lenPP  = len(PrefixPersistent)
	lenB   = len(PrefixBackward)
	lenBD  = len(PrefixBackwardDownstream)
)

// **Using empty string as key or value is not support.**

// TransferForward converts transient values to transient-upstream values and filters out original transient-upstream values.
// It should be used before the context is passing from server to client.
func TransferForward(ctx context.Context) context.Context {
	if n := getNode(ctx); n != nil {
		return withNode(ctx, n.transferForward())
	}
	return ctx
}

// GetValue retrieves the value set into the context by the given key.
func GetValue(ctx context.Context, k string) (v string, ok bool) {
	if n := getNode(ctx); n != nil {
		if idx, ok := search(n.transient, k); ok {
			return n.transient[idx].val, true
		}
		if idx, ok := search(n.stale, k); ok {
			return n.stale[idx].val, true
		}
	}
	return
}

// GetAllValues retrieves all transient values
func GetAllValues(ctx context.Context) (m map[string]string) {
	if n := getNode(ctx); n != nil {
		if cnt := len(n.stale) + len(n.transient); cnt > 0 {
			m = make(map[string]string, cnt)
			for _, kv := range n.stale {
				m[kv.key] = kv.val
			}
			for _, kv := range n.transient {
				m[kv.key] = kv.val
			}
		}
	}
	return
}

// WithValue sets the value into the context by the given key.
// This value will be propagated to the next service/endpoint through an RPC call.
//
// Notice that it will not propagate any further beyond the next service/endpoint,
// Use WithPersistentValue if you want to pass a key/value pair all the way.
func WithValue(ctx context.Context, k, v string) context.Context {
	if len(k) == 0 || len(v) == 0 {
		return ctx
	}
	if n := getNode(ctx); n != nil {
		if m := n.addTransient(k, v); m != n {
			return withNode(ctx, m)
		}
	} else {
		return withNode(ctx, &node{
			transient: []kv{{key: k, val: v}},
		})
	}
	return ctx
}

// DelValue deletes a key/value from the current context.
// Since empty string value is not valid, we could just set the value to be empty.
func DelValue(ctx context.Context, k string) context.Context {
	if len(k) == 0 {
		return ctx
	}
	if n := getNode(ctx); n != nil {
		if m := n.delTransient(k); m != n {
			return withNode(ctx, m)
		}
	}
	return ctx
}

// GetPersistentValue retrieves the persistent value set into the context by the given key.
func GetPersistentValue(ctx context.Context, k string) (v string, ok bool) {
	if n := getNode(ctx); n != nil {
		if idx, ok := search(n.persistent, k); ok {
			return n.persistent[idx].val, true
		}
	}
	return
}

// GetAllPersistentValues retrieves all persistent values.
func GetAllPersistentValues(ctx context.Context) (m map[string]string) {
	if n := getNode(ctx); n != nil {
		if cnt := len(n.persistent); cnt > 0 {
			m = make(map[string]string, cnt)
			for _, kv := range n.persistent {
				m[kv.key] = kv.val
			}
		}
	}
	return
}

// WithPersistentValue sets the value info the context by the given key.
// This value will be propagated to the services along the RPC call chain.
func WithPersistentValue(ctx context.Context, k, v string) context.Context {
	if len(k) == 0 || len(v) == 0 {
		return ctx
	}
	if n := getNode(ctx); n != nil {
		if m := n.addPersistent(k, v); m != n {
			return withNode(ctx, m)
		}
	} else {
		return withNode(ctx, &node{
			persistent: []kv{{key: k, val: v}},
		})
	}
	return ctx
}

// DelPersistentValue deletes a persistent key/value from the current context.
// Since empty string value is not valid, we could just set the value to be empty.
func DelPersistentValue(ctx context.Context, k string) context.Context {
	if len(k) == 0 {
		return ctx
	}
	if n := getNode(ctx); n != nil {
		if m := n.delPersistent(k); m != n {
			return withNode(ctx, m)
		}
	}
	return ctx
}
