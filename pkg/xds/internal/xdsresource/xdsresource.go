package xdsresource

import "time"

type Resource interface{}

type ResourceMeta struct {
	Version        string
	LastAccessTime time.Time
	UpdateTime     time.Time
}

type ResourceType int

const (
	UnknownResource ResourceType = iota
	ListenerType
	RouteConfigType
	ClusterType
	EndpointsType

	NameTableType
)

// Resource types in xDS v3.
const (
	apiTypePrefix          = "type.googleapis.com/"
	ListenerTypeUrl        = apiTypePrefix + "envoy.config.listener.v3.Listener"
	RouteTypeUrl           = apiTypePrefix + "envoy.config.route.v3.RouteConfiguration"
	ClusterTypeUrl         = apiTypePrefix + "envoy.config.cluster.v3.Cluster"
	EndpointTypeUrl        = apiTypePrefix + "envoy.config.endpoint.v3.ClusterLoadAssignment"
	SecretTypeUrl          = apiTypePrefix + "envoy.extensions.transport_sockets.tls.v3.Secret"
	ExtensionConfigTypeUrl = apiTypePrefix + "envoy.config.core.v3.TypedExtensionConfig"
	RuntimeTypeUrl         = apiTypePrefix + "envoy.service.runtime.v3.Runtime"
	HTTPConnManagerTypeUrl = apiTypePrefix + "envoy.extensions.filters.network.http_connection_manager.v3.HttpConnectionManager"
	ThriftProxyTypeUrl     = apiTypePrefix + "envoy.extensions.filters.network.thrift_proxy.v3.ThriftProxy"

	NameTableTypeUrl = apiTypePrefix + "istio.networking.nds.v1.NameTable"
	// AnyType is used only by ADS
	AnyType = ""
)

var ResourceTypeToUrl = map[ResourceType]string{
	ListenerType:    ListenerTypeUrl,
	RouteConfigType: RouteTypeUrl,
	ClusterType:     ClusterTypeUrl,
	EndpointsType:   EndpointTypeUrl,
	NameTableType:   NameTableTypeUrl,
}

var ResourceUrlToType = map[string]ResourceType{
	ListenerTypeUrl:  ListenerType,
	RouteTypeUrl:     RouteConfigType,
	ClusterTypeUrl:   ClusterType,
	EndpointTypeUrl:  EndpointsType,
	NameTableTypeUrl: NameTableType,
}

var ResourceTypeToName = map[ResourceType]string{
	ListenerType:    "Listener",
	RouteConfigType: "RouteConfig",
	ClusterType:     "Cluster",
	EndpointsType:   "ClusterLoadAssignment",
	NameTableType:   "NameTable",
}
