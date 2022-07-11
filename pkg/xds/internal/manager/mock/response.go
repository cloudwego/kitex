package mock

import (
	"github.com/cloudwego/kitex/pkg/xds/internal/api/discoveryv3"
	"github.com/cloudwego/kitex/pkg/xds/internal/xdsresource"
	"github.com/golang/protobuf/ptypes/any"
)

// LDS Response
var (
	LDSVersion1 = "v1"
	LDSVersion2 = "v2"
	LDSVersion3 = "v3"
	LDSNonce1   = "nonce1"
	LDSNonce2   = "nonce2"
	LDSNonce3   = "nonce3"

	LdsResp1 = &discoveryv3.DiscoveryResponse{
		VersionInfo: LDSVersion1,
		Resources: []*any.Any{
			xdsresource.MarshalAny(xdsresource.Listener1),
			xdsresource.MarshalAny(xdsresource.Listener2),
		},
		Canary:       false,
		TypeUrl:      xdsresource.ListenerTypeUrl,
		Nonce:        LDSNonce1,
		ControlPlane: nil,
	}
	LdsResp2 = &discoveryv3.DiscoveryResponse{
		VersionInfo: LDSVersion2,
		Resources: []*any.Any{
			xdsresource.MarshalAny(xdsresource.Listener2),
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
)

var (
	RDSVersion1 = "v1"
	RDSVersion2 = "v2"
	RDSNonce1   = "nonce1"
	RDSNonce2   = "nonce2"

	RdsResp1 = &discoveryv3.DiscoveryResponse{
		VersionInfo: RDSVersion1,
		Resources: []*any.Any{
			xdsresource.MarshalAny(xdsresource.RouteConfig1),
			xdsresource.MarshalAny(xdsresource.RouteConfig2),
		},
		Canary:       false,
		TypeUrl:      xdsresource.RouteTypeUrl,
		Nonce:        RDSNonce1,
		ControlPlane: nil,
	}
	RdsResp2 = &discoveryv3.DiscoveryResponse{
		VersionInfo: RDSVersion2,
		Resources: []*any.Any{
			xdsresource.MarshalAny(xdsresource.RouteConfig2),
		},
		Canary:       false,
		TypeUrl:      xdsresource.RouteTypeUrl,
		Nonce:        RDSNonce2,
		ControlPlane: nil,
	}
)

// CDS Response
var (
	CDSVersion1 = "v1"
	CDSVersion2 = "v2"
	CDSNonce1   = "nonce1"
	CDSNonce2   = "nonce2"

	CdsResp1 = &discoveryv3.DiscoveryResponse{
		VersionInfo: CDSVersion1,
		Resources: []*any.Any{
			xdsresource.MarshalAny(xdsresource.Cluster1),
			xdsresource.MarshalAny(xdsresource.Cluster2),
		},
		Canary:       false,
		TypeUrl:      xdsresource.ClusterTypeUrl,
		Nonce:        CDSNonce1,
		ControlPlane: nil,
	}
	CdsResp2 = &discoveryv3.DiscoveryResponse{
		VersionInfo: CDSVersion2,
		Resources: []*any.Any{
			xdsresource.MarshalAny(xdsresource.Cluster2),
		},
		Canary:       false,
		TypeUrl:      xdsresource.ClusterTypeUrl,
		Nonce:        CDSNonce2,
		ControlPlane: nil,
	}
)

// EDS Response
var (
	EDSVersion1 = "v1"
	EDSVersion2 = "v2"
	EDSNonce1   = "nonce1"
	EDSNonce2   = "nonce2"

	EdsResp1 = &discoveryv3.DiscoveryResponse{
		VersionInfo: EDSVersion1,
		Resources: []*any.Any{
			xdsresource.MarshalAny(xdsresource.Endpoints1),
			xdsresource.MarshalAny(xdsresource.Endpoints2),
		},
		Canary:       false,
		TypeUrl:      xdsresource.EndpointTypeUrl,
		Nonce:        EDSNonce1,
		ControlPlane: nil,
	}
	EdsResp2 = &discoveryv3.DiscoveryResponse{
		VersionInfo: EDSVersion2,
		Resources: []*any.Any{
			xdsresource.MarshalAny(xdsresource.Endpoints2),
		},
		Canary:       false,
		TypeUrl:      xdsresource.EndpointTypeUrl,
		Nonce:        EDSNonce2,
		ControlPlane: nil,
	}
)

// NDS response
var (
	NDSVersion1 = "v1"
	NDSNonce1   = "nonce1"
	NdsResp1    = &discoveryv3.DiscoveryResponse{
		VersionInfo: NDSVersion1,
		Resources: []*any.Any{
			xdsresource.MarshalAny(xdsresource.NameTable1),
		},
		Canary:       false,
		TypeUrl:      xdsresource.NameTableTypeUrl,
		Nonce:        NDSNonce1,
		ControlPlane: nil,
	}
)
