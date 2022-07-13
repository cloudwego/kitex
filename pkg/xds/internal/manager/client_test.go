package manager

import (
	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/xds/internal/api/discoveryv3"
	"github.com/cloudwego/kitex/pkg/xds/internal/manager/mock"
	"github.com/cloudwego/kitex/pkg/xds/internal/testutil/resource"
	"github.com/cloudwego/kitex/pkg/xds/internal/xdsresource"
	v3core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	"reflect"
	"testing"
)

type mockStreamClient struct {
	StreamClient
}

func (sc *mockStreamClient) Send(*discoveryv3.DiscoveryRequest) error {
	return nil
}

func (sc *mockStreamClient) Recv() (*discoveryv3.DiscoveryResponse, error) {
	select {}
}

func Test_newXdsClient(t *testing.T) {
	address := ":8888"
	svr := mock.StartXDSServer(address)
	defer func() {
		if svr != nil {
			_ = svr.Stop()
		}
	}()
	_, err := newXdsClient(
		&BootstrapConfig{
			node:      &v3core.Node{},
			xdsSvrCfg: &XDSServerConfig{serverAddress: address},
		},
		nil,
	)
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
	c := &xdsClient{
		config: &BootstrapConfig{
			node:      NodeProto,
			xdsSvrCfg: XdsServerConfig,
		},
		streamClient:    &mockStreamClient{},
		watchedResource: make(map[xdsresource.ResourceType]map[string]bool),
		versionMap:      make(map[xdsresource.ResourceType]string),
		nonceMap:        make(map[xdsresource.ResourceType]string),
		resourceUpdater: &xdsResourceManager{
			cache:       make(map[xdsresource.ResourceType]map[string]xdsresource.Resource),
			meta:        make(map[xdsresource.ResourceType]map[string]*xdsresource.ResourceMeta),
			notifierMap: make(map[xdsresource.ResourceType]map[string][]*notifier),
			dumpPath:    defaultDumpPath,
		},
		refreshInterval: defaultRefreshInterval,
	}
	// handle the response that includes resources that are not in the subscribed list
	err := c.handleResponse(resource.LdsResp1)
	test.Assert(t, err == nil)
	test.Assert(t, c.versionMap[xdsresource.ListenerType] == resource.LDSVersion1)
	test.Assert(t, c.nonceMap[xdsresource.ListenerType] == resource.LDSNonce1)
	test.Assert(t, c.watchedResource[xdsresource.RouteConfigType][resource.RouteConfigName1] == false)
	c.watchedResource[xdsresource.ListenerType] = map[string]bool{
		resource.ListenerName1: true,
	}
	err = c.handleResponse(resource.LdsResp1)
	test.Assert(t, c.watchedResource[xdsresource.RouteConfigType][resource.RouteConfigName1] == true)
}
