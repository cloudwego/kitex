package resource

import (
	"github.com/cloudwego/kitex/pkg/xds/internal/api/discoveryv3"
	"github.com/cloudwego/kitex/pkg/xds/internal/testutil"
	"github.com/cloudwego/kitex/pkg/xds/internal/xdsresource"
	v3clusterpb "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	v3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	v3endpointpb "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	v3listenerpb "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	v3routepb "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	v3httppb "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	"github.com/golang/protobuf/ptypes/any"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// listener
var (
	ListenerName1 = "listener1"
	ListenerName2 = "listener2"
	LDSVersion1   = "v1"
	LDSVersion2   = "v2"
	LDSVersion3   = "v3"
	LDSNonce1     = "nonce1"
	LDSNonce2     = "nonce2"
	LDSNonce3     = "nonce3"

	RouteConfigName1    = "routeConfig1"
	RouteConfigName2    = "routeConfig2"
	RouteConfigVersion1 = "v1"
	RouteConfigVersion2 = "v2"
	RouteConfigNonce1   = "nonce1"
	RouteConfigNonce2   = "nonce2"

	ClusterName     = "cluster"
	ClusterVersion  = "v3"
	ClusterNonce    = "nonce3"
	EndpointName    = "endpoint"
	EndpointVersion = "v4"
	EndpointNonce   = "nonce4"

	Listener1 = &v3listenerpb.Listener{
		Name: ListenerName1,
		ApiListener: &v3listenerpb.ApiListener{
			ApiListener: testutil.MarshalAny(
				&v3httppb.HttpConnectionManager{
					RouteSpecifier: &v3httppb.HttpConnectionManager_Rds{
						Rds: &v3httppb.Rds{
							RouteConfigName: RouteConfigName1,
						},
					},
				},
			),
		},
	}
	Listener2 = &v3listenerpb.Listener{
		Name: ListenerName2,
		ApiListener: &v3listenerpb.ApiListener{
			ApiListener: testutil.MarshalAny(
				&v3httppb.HttpConnectionManager{
					RouteSpecifier: &v3httppb.HttpConnectionManager_Rds{
						Rds: &v3httppb.Rds{
							RouteConfigName: RouteConfigName1,
						},
					},
				},
			),
		},
	}
	RouteConfig1 = &v3routepb.RouteConfiguration{
		Name: RouteConfigName1,
		VirtualHosts: []*v3routepb.VirtualHost{
			{
				Name: "vhName",
				Routes: []*v3routepb.Route{
					{
						Match: &v3routepb.RouteMatch{
							PathSpecifier: &v3routepb.RouteMatch_Path{
								Path: "path",
							},
						},
						Action: &v3routepb.Route_Route{
							Route: &v3routepb.RouteAction{
								ClusterSpecifier: &v3routepb.RouteAction_WeightedClusters{
									WeightedClusters: &v3routepb.WeightedCluster{
										Clusters: []*v3routepb.WeightedCluster_ClusterWeight{
											{
												Name: ClusterName,
												Weight: &wrapperspb.UInt32Value{
													Value: uint32(50),
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	RouteConfig2 = &v3routepb.RouteConfiguration{
		Name: RouteConfigName2,
		VirtualHosts: []*v3routepb.VirtualHost{
			{
				Name: "vhName",
				Routes: []*v3routepb.Route{
					{
						Match: &v3routepb.RouteMatch{
							PathSpecifier: &v3routepb.RouteMatch_Path{
								Path: "path",
							},
						},
						Action: &v3routepb.Route_Route{
							Route: &v3routepb.RouteAction{
								ClusterSpecifier: &v3routepb.RouteAction_WeightedClusters{
									WeightedClusters: &v3routepb.WeightedCluster{
										Clusters: []*v3routepb.WeightedCluster_ClusterWeight{
											{
												Name: ClusterName,
												Weight: &wrapperspb.UInt32Value{
													Value: uint32(50),
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	Cluster1 = &v3clusterpb.Cluster{
		Name:                 ClusterName,
		ClusterDiscoveryType: &v3clusterpb.Cluster_Type{Type: v3clusterpb.Cluster_EDS},
		EdsClusterConfig: &v3clusterpb.Cluster_EdsClusterConfig{
			ServiceName: EndpointName,
		},
		LbPolicy:       v3clusterpb.Cluster_ROUND_ROBIN,
		LoadAssignment: nil,
	}
	addr             = "127.0.0.1"
	port1, port2     = 8080, 8081
	weight1, weight2 = 50, 50
	Endpoints1       = &v3endpointpb.ClusterLoadAssignment{
		ClusterName: EndpointName,
		Endpoints: []*v3endpointpb.LocalityLbEndpoints{
			{
				LbEndpoints: []*v3endpointpb.LbEndpoint{
					{
						HostIdentifier: &v3endpointpb.LbEndpoint_Endpoint{
							Endpoint: &v3endpointpb.Endpoint{
								Address: &v3.Address{
									Address: &v3.Address_SocketAddress{
										SocketAddress: &v3.SocketAddress{
											Protocol: v3.SocketAddress_TCP,
											Address:  addr,
											PortSpecifier: &v3.SocketAddress_PortValue{
												PortValue: uint32(port1),
											},
										},
									},
								},
							},
						},
						LoadBalancingWeight: &wrapperspb.UInt32Value{
							Value: uint32(weight1),
						},
					},
					{
						HostIdentifier: &v3endpointpb.LbEndpoint_Endpoint{
							Endpoint: &v3endpointpb.Endpoint{
								Address: &v3.Address{
									Address: &v3.Address_SocketAddress{
										SocketAddress: &v3.SocketAddress{
											Protocol: v3.SocketAddress_TCP,
											Address:  addr,
											PortSpecifier: &v3.SocketAddress_PortValue{
												PortValue: uint32(port2),
											},
										},
									},
								},
							},
						},
						LoadBalancingWeight: &wrapperspb.UInt32Value{
							Value: uint32(weight2),
						},
					},
				},
			},
		},
	}

	// response
	LdsResp1 = &discoveryv3.DiscoveryResponse{
		VersionInfo: LDSVersion1,
		Resources: []*any.Any{
			testutil.MarshalAny(Listener1),
			testutil.MarshalAny(Listener2),
		},
		Canary:       false,
		TypeUrl:      xdsresource.ListenerTypeUrl,
		Nonce:        LDSNonce1,
		ControlPlane: nil,
	}
	LdsResp2 = &discoveryv3.DiscoveryResponse{
		VersionInfo: LDSVersion2,
		Resources: []*any.Any{
			testutil.MarshalAny(Listener2),
		},
		Canary:       false,
		TypeUrl:      xdsresource.ListenerTypeUrl,
		Nonce:        LDSNonce2,
		ControlPlane: nil,
	}
	LdsResp3 = &discoveryv3.DiscoveryResponse{
		VersionInfo:  LDSVersion3,
		Resources:    nil,
		Canary:       false,
		TypeUrl:      xdsresource.ListenerTypeUrl,
		Nonce:        LDSNonce3,
		ControlPlane: nil,
	}

	RdsResp1 = &discoveryv3.DiscoveryResponse{
		VersionInfo: RouteConfigVersion1,
		Resources: []*any.Any{
			testutil.MarshalAny(RouteConfig1),
			testutil.MarshalAny(RouteConfig2),
		},
		Canary:       false,
		TypeUrl:      xdsresource.RouteTypeUrl,
		Nonce:        RouteConfigNonce1,
		ControlPlane: nil,
	}
	RdsResp2 = &discoveryv3.DiscoveryResponse{
		VersionInfo: RouteConfigVersion1,
		Resources: []*any.Any{
			testutil.MarshalAny(RouteConfig2),
		},
		Canary:       false,
		TypeUrl:      xdsresource.RouteTypeUrl,
		Nonce:        RouteConfigNonce1,
		ControlPlane: nil,
	}
	CdsResp1 = &discoveryv3.DiscoveryResponse{
		VersionInfo: ClusterVersion,
		Resources: []*any.Any{
			testutil.MarshalAny(Cluster1),
		},
		Canary:       false,
		TypeUrl:      xdsresource.ClusterTypeUrl,
		Nonce:        ClusterNonce,
		ControlPlane: nil,
	}
	EdsResp1 = &discoveryv3.DiscoveryResponse{
		VersionInfo: EndpointVersion,
		Resources: []*any.Any{
			testutil.MarshalAny(Endpoints1),
		},
		Canary:       false,
		TypeUrl:      xdsresource.EndpointTypeUrl,
		Nonce:        EndpointNonce,
		ControlPlane: nil,
	}
)
