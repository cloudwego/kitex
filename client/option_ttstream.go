/*
 * Copyright 2025 CloudWeGo Authors
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

package client

import (
	"fmt"

	"github.com/cloudwego/kitex/internal/client"
	"github.com/cloudwego/kitex/pkg/remote/trans/ttstream"
	"github.com/cloudwego/kitex/pkg/utils"
)

// WithTTHeaderStreamingOptions add ttheader streaming options for client.
func WithTTHeaderStreamingOptions(opts ...TTHeaderStreamingOption) Option {
	return Option{F: func(o *client.Options, di *utils.Slice) {
		var udi utils.Slice
		for _, opt := range opts {
			opt.F(&o.TTHeaderStreamingOptions, &udi)
		}
		di.Push(map[string]interface{}{
			"WithTTHeaderStreamingOptions": udi,
		})
	}}
}

// WithTTHeaderStreamingTransportOptions add ttheader streaming transport options for client.
func WithTTHeaderStreamingTransportOptions(opt ...ttstream.ClientProviderOption) TTHeaderStreamingOption {
	return TTHeaderStreamingOption{F: func(o *client.TTHeaderStreamingOptions, di *utils.Slice) {
		di.Push(fmt.Sprintf("WithTTHeaderStreamingProviderOption(%T)", opt))

		o.TransportOptions = append(o.TransportOptions, opt...)
	}}
}

// WithTTHeaderStreamingMaxReceiveMessageSize sets the maximum size in bytes of a received TTHeader Streaming frame payload.
// A non-positive value means unlimited.
func WithTTHeaderStreamingMaxReceiveMessageSize(s int) Option {
	return Option{F: func(o *client.Options, di *utils.Slice) {
		di.Push(fmt.Sprintf("WithTTHeaderStreamingMaxReceiveMessageSize(%d)", s))
		o.TTHeaderStreamingOptions.TransportOptions = append(
			o.TTHeaderStreamingOptions.TransportOptions,
			ttstream.WithClientMaxReceiveMessageSize(s),
		)
	}}
}
