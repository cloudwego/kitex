package manager

import (
	"context"
	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/xds/internal/manager/mock"
	"github.com/cloudwego/kitex/pkg/xds/internal/testutil/resource"
	"github.com/cloudwego/kitex/pkg/xds/internal/xdsresource"
	v3core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	"testing"
	"time"
)

// for test use
var (
	XdsSercerAddress = ":8888"
	NodeProto        = &v3core.Node{
		Id: "sidecar~kitex-test-node",
	}
	XdsServerConfig = &XDSServerConfig{
		serverAddress: XdsSercerAddress,
	}
	XdsBootstrapConfig = &BootstrapConfig{
		node:      NodeProto,
		xdsSvrCfg: XdsServerConfig,
	}
)

func Test_xdsResourceManager_Get(t *testing.T) {
	// Init
	svr := mock.StartXDSServer(XdsSercerAddress)
	defer func() {
		if svr != nil {
			_ = svr.Stop()
		}
	}()
	m, err := NewXDSResourceManager(XdsBootstrapConfig)
	test.Assert(t, err == nil)

	type args struct {
		ctx          context.Context
		resourceType xdsresource.ResourceType
		resourceName string
	}
	tests := []struct {
		name    string
		args    args
		want    interface{}
		wantErr bool
	}{
		{
			name: "UnknownResourceType",
			args: args{
				ctx:          context.Background(),
				resourceType: 10,
				resourceName: "",
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "GetListenerSuccess",
			args: args{
				ctx:          context.Background(),
				resourceType: xdsresource.ListenerType,
				resourceName: resource.ListenerName1,
			},
			want:    nil,
			wantErr: false,
		},
		{
			name: "GetRouteConfigSuccess",
			args: args{
				ctx:          context.Background(),
				resourceType: xdsresource.RouteConfigType,
				resourceName: resource.RouteConfigName1,
			},
			want:    nil,
			wantErr: false,
		},
		{
			name: "GetClusterSuccess",
			args: args{
				ctx:          context.Background(),
				resourceType: xdsresource.ClusterType,
				resourceName: resource.ClusterName,
			},
			want:    nil,
			wantErr: false,
		},
		{
			name: "GetEndpointsSuccess",
			args: args{
				ctx:          context.Background(),
				resourceType: xdsresource.EndpointsType,
				resourceName: resource.EndpointName,
			},
			want:    nil,
			wantErr: false,
		},
		{
			name: "ListenerNotExist",
			args: args{
				ctx:          context.Background(),
				resourceType: xdsresource.ListenerType,
				resourceName: "random",
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "RouteConfigNotExist",
			args: args{
				ctx:          context.Background(),
				resourceType: xdsresource.RouteConfigType,
				resourceName: "random",
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "ClusterNotExist",
			args: args{
				ctx:          context.Background(),
				resourceType: xdsresource.ClusterType,
				resourceName: "random",
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "EndpointsNotExist",
			args: args{
				ctx:          context.Background(),
				resourceType: xdsresource.EndpointsType,
				resourceName: "random",
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			_, err := m.Get(tt.args.ctx, tt.args.resourceType, tt.args.resourceName)
			if (err != nil) != tt.wantErr {
				t.Errorf("Get() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func Test_xdsResourceManager_Get_Resource_Update(t *testing.T) {
	// Init
	svr := mock.StartXDSServer(XdsSercerAddress)
	defer func() {
		if svr != nil {
			_ = svr.Stop()
		}
	}()
	m, initErr := NewXDSResourceManager(XdsBootstrapConfig)
	test.Assert(t, initErr == nil)

	var res xdsresource.Resource
	var err error
	// GetListener
	res, err = m.Get(context.Background(), xdsresource.ListenerType, resource.ListenerName1)
	test.Assert(t, err == nil)
	test.Assert(t, res != nil)
	test.Assert(t, m.meta[xdsresource.ListenerType][resource.ListenerName1] != nil)
	test.Assert(t, res.(*xdsresource.ListenerResource).RouteConfigName == resource.RouteConfigName1)
	// push the new resource and check if the resourceManager can update the resource
	svr.PushResourceUpdate(resource.LdsResp2)
	time.Sleep(time.Millisecond * 100)
	res, err = m.Get(context.Background(), xdsresource.ListenerType, resource.ListenerName1)
	test.Assert(t, err != nil)

	// GetRouteConfig
	res, err = m.Get(context.Background(), xdsresource.RouteConfigType, resource.RouteConfigName1)
	test.Assert(t, err == nil)
	test.Assert(t, res != nil)
	// push the new resource and check if the resourceManager can update the resource
	svr.PushResourceUpdate(resource.RdsResp2)
	time.Sleep(time.Millisecond * 100)
	res, err = m.Get(context.Background(), xdsresource.RouteConfigType, resource.RouteConfigName1)
	test.Assert(t, err != nil)
}

func Test_xdsResourceManager_getFromCache(t *testing.T) {
	m := &xdsResourceManager{
		cache: map[xdsresource.ResourceType]map[string]xdsresource.Resource{
			xdsresource.ListenerType: {
				resource.ListenerName1: resource.Listener1,
			},
		},
		meta: map[xdsresource.ResourceType]map[string]*xdsresource.ResourceMeta{
			xdsresource.ListenerType: {},
		},
	}

	// succeed
	res, ok := m.getFromCache(xdsresource.ListenerType, resource.ListenerName1)
	test.Assert(t, ok == true)
	test.Assert(t, res == resource.Listener1)
	test.Assert(t, m.meta[xdsresource.ListenerType][resource.ListenerName1] != nil)

	// failed
	res, ok = m.getFromCache(xdsresource.ListenerType, "randomListener")
	test.Assert(t, ok == false)
	res, ok = m.getFromCache(xdsresource.ClusterType, "randomCluster")
	test.Assert(t, ok == false)
}
