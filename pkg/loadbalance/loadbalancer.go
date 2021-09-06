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

package loadbalance

import (
	"context"

	"github.com/jackedelic/kitex/internal/pkg/discovery"
)

// Picker picks an instance for next RPC call.
type Picker interface {
	Next(ctx context.Context, request interface{}) discovery.Instance
}

// Loadbalancer generates pickers for the given service discovery result.
type Loadbalancer interface {
	GetPicker(discovery.Result) Picker
	Name() string // unique key
}

// Rebalancer is a kind of Loadbalancer that performs rebalancing when the result of service discovery changes.
type Rebalancer interface {
	Rebalance(discovery.Change)
	Delete(discovery.Change)
}

// SynthesizedPicker synthesizes a picker using a next function.
type SynthesizedPicker struct {
	NextFunc func(ctx context.Context, request interface{}) discovery.Instance
}

// Next implements the Picker interface.
func (sp *SynthesizedPicker) Next(ctx context.Context, request interface{}) discovery.Instance {
	return sp.NextFunc(ctx, request)
}

// SynthesizedLoadbalancer synthesizes a loadbalancer using a GetPickerFunc function.
type SynthesizedLoadbalancer struct {
	GetPickerFunc func(discovery.Result) Picker
	NameFunc      func() string
}

// GetPicker implements the Loadbalancer interface.
func (slb *SynthesizedLoadbalancer) GetPicker(entry discovery.Result) Picker {
	if slb.GetPickerFunc == nil {
		return nil
	}
	return slb.GetPickerFunc(entry)
}

// Name implements the Loadbalancer interface.
func (slb *SynthesizedLoadbalancer) Name() string {
	if slb.NameFunc == nil {
		return ""
	}
	return slb.NameFunc()
}
