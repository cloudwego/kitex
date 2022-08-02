package mock

import (
	"fmt"
	"github.com/cloudwego/kitex/pkg/xds/internal/api/discoveryv3"
	"github.com/cloudwego/kitex/pkg/xds/internal/api/discoveryv3/aggregateddiscoveryservice"
	"github.com/cloudwego/kitex/pkg/xds/internal/xdsresource"
	"github.com/cloudwego/kitex/server"
	"net"
	"sync"
	"time"
)

type testXDSServer struct {
	mu sync.Mutex
	server.Server
	respCh chan *discoveryv3.DiscoveryResponse
}

func (svr *testXDSServer) PushResourceUpdate(resp *discoveryv3.DiscoveryResponse) {
	svr.respCh <- resp
}

func StartXDSServer(address string) *testXDSServer {
	addr, _ := net.ResolveTCPAddr("tcp", address)
	respCh := make(chan *discoveryv3.DiscoveryResponse)

	svr := aggregateddiscoveryservice.NewServer(
		&testAdsService{
			respCh: respCh,
			resourceCache: map[xdsresource.ResourceType]map[string]*discoveryv3.DiscoveryResponse{
				xdsresource.ListenerType: {
					"":                   LdsResp1,
					LdsResp1.VersionInfo: LdsResp1,
					LdsResp2.VersionInfo: LdsResp2,
					LdsResp3.VersionInfo: LdsResp3,
				},
				xdsresource.RouteConfigType: {
					"":                   RdsResp1,
					RdsResp1.VersionInfo: RdsResp1,
				},
				xdsresource.ClusterType: {
					"":                   CdsResp1,
					CdsResp1.VersionInfo: CdsResp1,
				},
				xdsresource.EndpointsType: {
					"":                   EdsResp1,
					EdsResp1.VersionInfo: EdsResp1,
				},
				xdsresource.NameTableType: {
					"": NdsResp1,
				},
			},
		},
		server.WithServiceAddr(addr),
	)

	xdsServer := &testXDSServer{
		sync.Mutex{},
		svr,
		respCh,
	}
	go func() {
		_ = xdsServer.Run()
	}()
	time.Sleep(time.Millisecond * 100)
	return xdsServer
}

type testAdsService struct {
	respCh chan *discoveryv3.DiscoveryResponse
	// resourceCache: version -> response
	resourceCache map[xdsresource.ResourceType]map[string]*discoveryv3.DiscoveryResponse
}

func (svr *testAdsService) StreamAggregatedResources(stream discoveryv3.AggregatedDiscoveryService_StreamAggregatedResourcesServer) (err error) {
	errCh := make(chan error, 2)
	stopCh := make(chan struct{})
	defer close(stopCh)
	// receive the request
	go func() {
		for {
			select {
			case <-stopCh:
				return
			default:
			}

			msg, err := stream.Recv()
			if err != nil {
				errCh <- err
				return
			}
			svr.handleRequest(msg)
		}
	}()

	// send the response
	go func() {
		for {
			select {
			case <-stopCh:
				return
			default:
			}

			select {
			case resp := <-svr.respCh:
				err := stream.Send(resp)
				if err != nil {
					errCh <- err
					return
				}
			}
		}
	}()

	select {
	case err := <-errCh:
		return err
	}
}

func (svr *testAdsService) handleRequest(msg interface{}) {
	req, ok := msg.(*discoveryv3.DiscoveryRequest)
	if !ok {
		// ignore msgs other than DiscoveryRequest
		return
	}
	rType, ok := xdsresource.ResourceUrlToType[req.TypeUrl]
	if !ok {
		return
	}
	if _, ok := svr.resourceCache[rType]; !ok {
		return
	}
	cache, ok := svr.resourceCache[rType][req.VersionInfo]
	// ignore ack
	if !ok || req.ResponseNonce == cache.Nonce {
		return
	}
	svr.respCh <- cache
}

func (svr *testAdsService) DeltaAggregatedResources(stream discoveryv3.AggregatedDiscoveryService_DeltaAggregatedResourcesServer) (err error) {
	return fmt.Errorf("DeltaAggregatedResources has not been implemented")
}
