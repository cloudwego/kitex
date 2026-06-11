/*
 * Copyright 2026 CloudWeGo Authors
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

package transmeta

import (
	"context"
	"sync/atomic"

	"github.com/bytedance/gopkg/cloud/metainfo"
)

// MetainfoPropagationMode controls transient metainfo propagation semantics.
//
// Deprecated: this is a temporary rollout switch for metainfo single-hop
// semantics and will be removed once single-hop becomes the default behavior.
type MetainfoPropagationMode int32

const (
	// MetainfoPropagationLegacy keeps the historical behavior: stale/upstream
	// transient values are serialized again and may leak beyond one hop.
	//
	// Deprecated: this is part of the temporary metainfo propagation rollout and
	// will be removed once single-hop becomes the default behavior.
	MetainfoPropagationLegacy MetainfoPropagationMode = iota
	// MetainfoPropagationSingleHop only serializes current-hop transient values and
	// persistent values are serialized to downstream calls.
	//
	// Deprecated: this is part of the temporary metainfo propagation rollout and
	// will be removed once single-hop becomes the default behavior.
	MetainfoPropagationSingleHop
)

var (
	metainfoPropagationMode atomic.Int32
	// metainfoPropagationLegacyOverride is an emergency kill switch. Only an
	// explicitly configured legacy value overrides the context-aware provider; single_hop
	// remains provider-controlled for service-level rollout.
	metainfoPropagationLegacyOverride atomic.Bool
	metainfoPropagationModeProvider   atomic.Value // stores func(context.Context) MetainfoPropagationMode
)

func init() {
	setMetainfoPropagationMode(MetainfoPropagationLegacy)
	metainfoPropagationLegacyOverride.Store(false)
	SetMetainfoPropagationModeProvider(nil)
}

// SetMetainfoPropagationMode sets the process default mode. It is primarily
// intended for tests and environments without a dynamic config center. Setting
// legacy also acts as an emergency kill switch that overrides the provider;
// setting single_hop only changes the process default and lets the provider
// continue to make context-aware decisions.
//
// Deprecated: this is a temporary compatibility switch for the metainfo
// single-hop rollout and will be removed once single-hop becomes the default
// behavior.
func SetMetainfoPropagationMode(mode MetainfoPropagationMode) {
	mode = normalizeMetainfoPropagationMode(mode)
	setMetainfoPropagationMode(mode)
	metainfoPropagationLegacyOverride.Store(mode == MetainfoPropagationLegacy)
}

func setMetainfoPropagationMode(mode MetainfoPropagationMode) {
	metainfoPropagationMode.Store(int32(normalizeMetainfoPropagationMode(mode)))
}

// GetMetainfoPropagationMode returns the process default mode.
//
// Deprecated: this is part of the temporary metainfo propagation rollout and
// will be removed once single-hop becomes the default behavior.
func GetMetainfoPropagationMode() MetainfoPropagationMode {
	return MetainfoPropagationMode(metainfoPropagationMode.Load())
}

// SetMetainfoPropagationModeProvider installs a context-aware mode provider.
// Internal distributions can use it to bridge their config center into the
// open-source transmeta handler without adding config-center dependencies here.
// Passing nil resets the provider to the process default mode.
//
// Deprecated: this is part of the temporary metainfo propagation rollout and
// will be removed once single-hop becomes the default behavior.
func SetMetainfoPropagationModeProvider(provider func(context.Context) MetainfoPropagationMode) {
	if provider == nil {
		provider = func(context.Context) MetainfoPropagationMode {
			return GetMetainfoPropagationMode()
		}
	}
	metainfoPropagationModeProvider.Store(provider)
}

func getMetainfoPropagationMode(ctx context.Context) MetainfoPropagationMode {
	if metainfoPropagationLegacyOverride.Load() {
		return GetMetainfoPropagationMode()
	}
	provider, _ := metainfoPropagationModeProvider.Load().(func(context.Context) MetainfoPropagationMode)
	if provider == nil {
		return GetMetainfoPropagationMode()
	}
	return normalizeMetainfoPropagationMode(provider(ctx))
}

// TransferForwardMetaInfo advances inbound metainfo only under single_hop mode.
// Legacy mode intentionally keeps the historical context unchanged so the
// rollout switch remains reversible.
//
// Deprecated: this is an internal rollout helper and will be removed once
// single-hop becomes the default behavior.
func TransferForwardMetaInfo(ctx context.Context) context.Context {
	if getMetainfoPropagationMode(ctx) == MetainfoPropagationSingleHop {
		return metainfo.TransferForward(ctx)
	}
	return ctx
}

func normalizeMetainfoPropagationMode(mode MetainfoPropagationMode) MetainfoPropagationMode {
	switch mode {
	case MetainfoPropagationSingleHop:
		return mode
	default:
		return MetainfoPropagationLegacy
	}
}

// SaveOutboundMetaInfoToMap serializes metainfo for outbound headers according
// to MetainfoPropagationMode.
//
// Deprecated: this is an internal rollout helper and will be removed once
// single-hop becomes the default behavior.
func SaveOutboundMetaInfoToMap(ctx context.Context, m map[string]string) {
	switch getMetainfoPropagationMode(ctx) {
	case MetainfoPropagationSingleHop:
		metainfo.SaveMetaInfoToMap(metainfo.TransferForward(ctx), m)
	default:
		metainfo.SaveMetaInfoToMap(ctx, m)
	}
}

// SaveOutboundMetaInfoToHTTPHeader serializes metainfo to outbound HTTP/2
// metadata according to MetainfoPropagationMode.
//
// Deprecated: this is an internal rollout helper and will be removed once
// single-hop becomes the default behavior.
func SaveOutboundMetaInfoToHTTPHeader(ctx context.Context, h metainfo.HTTPHeaderSetter) {
	switch getMetainfoPropagationMode(ctx) {
	case MetainfoPropagationSingleHop:
		metainfo.ToHTTPHeader(metainfo.TransferForward(ctx), h)
	default:
		metainfo.ToHTTPHeader(ctx, h)
	}
}
