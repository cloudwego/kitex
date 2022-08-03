package manager

import (
	"context"
	"github.com/cloudwego/kitex/client/callopt"
	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/xds/internal/api/discoveryv3"
	"github.com/cloudwego/kitex/pkg/xds/internal/manager/mock"
	"github.com/cloudwego/kitex/pkg/xds/internal/xdsresource"
	v3core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	"reflect"
	"sync"
	"testing"
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
			xdsSvrCfg: &XDSServerConfig{serverAddress: address},
		},
		nil,
	)
	defer c.close()
	test.Assert(t, err == nil)
}

func Test_xdsClient_prepareRequest(t *testing.T) {
	c := &xdsClient{
		config: XdsBootstrapConfig,
		watchedResource: map[xdsresource.ResourceType]map[string]bool{
			xdsresource.RouteConfigType: {},
			xdsresource.ClusterType:     {"cluster1": true},
		},
	}
	type args struct {
		resourceType xdsresource.ResourceType
		ack          bool
		errMsg       string
	}
	tests := []struct {
		name string
		args args
		want *discoveryv3.DiscoveryRequest
	}{
		{
			name: "unknown resource type",
			args: args{
				resourceType: -1,
				ack:          false,
				errMsg:       "",
			},
			want: nil,
		},
		{
			name: "resource type not in the subscribed list",
			args: args{
				resourceType: xdsresource.ListenerType,
				ack:          false,
				errMsg:       "",
			},
			want: nil,
		},
		{
			name: "resource list is empty",
			args: args{
				resourceType: xdsresource.RouteConfigType,
				ack:          false,
				errMsg:       "",
			},
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := c.prepareRequest(tt.args.resourceType, tt.args.ack, tt.args.errMsg); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("prepareRequest() = %v, want %v", got, tt.want)
			}
		})
	}

	var req *discoveryv3.DiscoveryRequest
	// ack = false
	req = c.prepareRequest(xdsresource.ClusterType, false, "")
	test.Assert(t, req != nil)
	test.Assert(t, req.ResourceNames[0] == "cluster1")
	test.Assert(t, req.ErrorDetail == nil)
	// ack = true, errMsg = ""
	req = c.prepareRequest(xdsresource.ClusterType, true, "")
	test.Assert(t, req != nil)
	test.Assert(t, req.ErrorDetail == nil)
	// ack = true, errMsg = "error"
	req = c.prepareRequest(xdsresource.ClusterType, true, "error")
	test.Assert(t, req != nil)
	test.Assert(t, req.ErrorDetail.Message == "error")
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
			mu:          sync.Mutex{},
			dumpPath:    defaultDumpPath,
		},
		refreshInterval: defaultRefreshInterval,
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
