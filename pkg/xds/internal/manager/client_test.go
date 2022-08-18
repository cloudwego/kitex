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

package manager

import (
	"context"
	"sync"
	"testing"

	"github.com/cloudwego/kitex/client/callopt"
	"github.com/cloudwego/kitex/internal/test"
	discoveryv3 "github.com/cloudwego/kitex/pkg/xds/internal/api/kitex_gen/envoy/service/discovery/v3"
	"github.com/cloudwego/kitex/pkg/xds/internal/manager/mock"
	"github.com/cloudwego/kitex/pkg/xds/internal/xdsresource"
	v3core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
)

type mockADSClient struct {
	ADSClient
}

func (sc *mockADSClient) StreamAggregatedResources(ctx context.Context, callOptions ...callopt.Option) (stream ADSStream, err error) {
	return &mockADSStream{}, nil
}

type mockADSStream struct {
	ADSStream
}

func (sc *mockADSStream) Send(*discoveryv3.DiscoveryRequest) error {
	return nil
}

func (sc *mockADSStream) Recv() (*discoveryv3.DiscoveryResponse, error) {
	return nil, nil
}

func (sc *mockADSStream) Close() error {
	return nil
}

func Test_newXdsClient(t *testing.T) {
	address := ":8888"
	svr := mock.StartXDSServer(address)
	defer func() {
		if svr != nil {
			_ = svr.Stop()
		}
	}()
	c, err := newXdsClient(
		&BootstrapConfig{
			node:      &v3core.Node{},
			xdsSvrCfg: &XDSServerConfig{SvrAddr: address},
		},
		nil,
	)
	defer c.close()
	test.Assert(t, err == nil)
}

func Test_xdsClient_handleResponse(t *testing.T) {
	// inject mock
	c := &xdsClient{
		config: &BootstrapConfig{
			node:      NodeProto,
			xdsSvrCfg: XdsServerConfig,
		},
		adsClient:       &mockADSClient{},
		watchedResource: make(map[xdsresource.ResourceType]map[string]bool),
		cipResolver:     newNdsResolver(),
		versionMap:      make(map[xdsresource.ResourceType]string),
		nonceMap:        make(map[xdsresource.ResourceType]string),
		resourceUpdater: &xdsResourceManager{
			cache:       map[xdsresource.ResourceType]map[string]xdsresource.Resource{},
			meta:        make(map[xdsresource.ResourceType]map[string]*xdsresource.ResourceMeta),
			notifierMap: make(map[xdsresource.ResourceType]map[string]*notifier),
			mu:          sync.RWMutex{},
			opts:        NewOptions(nil),
		},
		closeCh:         make(chan struct{}),
	}
	defer c.close()

	// handle before watch
	err := c.handleResponse(mock.LdsResp1)
	test.Assert(t, err == nil)
	test.Assert(t, c.versionMap[xdsresource.ListenerType] == "")
	test.Assert(t, c.nonceMap[xdsresource.ListenerType] == "")

	// handle after watch
	c.watchedResource[xdsresource.ListenerType] = make(map[string]bool)
	err = c.handleResponse(mock.LdsResp1)
	test.Assert(t, err == nil)
	test.Assert(t, c.versionMap[xdsresource.ListenerType] == mock.LDSVersion1)
	test.Assert(t, c.nonceMap[xdsresource.ListenerType] == mock.LDSNonce1)

	c.watchedResource[xdsresource.RouteConfigType] = make(map[string]bool)
	err = c.handleResponse(mock.RdsResp1)
	test.Assert(t, err == nil)
	test.Assert(t, c.versionMap[xdsresource.RouteConfigType] == mock.RDSVersion1)
	test.Assert(t, c.nonceMap[xdsresource.RouteConfigType] == mock.RDSNonce1)
}
