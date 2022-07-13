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
)

// Resource types in xDS v3.
const (
	apiTypePrefix          = "type.googleapis.com/"
	EndpointTypeUrl        = apiTypePrefix + "envoy.config.endpoint.v3.ClusterLoadAssignment"
	ClusterTypeUrl         = apiTypePrefix + "envoy.config.cluster.v3.Cluster"
	RouteTypeUrl           = apiTypePrefix + "envoy.config.route.v3.RouteConfiguration"
	ListenerTypeUrl        = apiTypePrefix + "envoy.config.listener.v3.Listener"
	SecretTypeUrl          = apiTypePrefix + "envoy.extensions.transport_sockets.tls.v3.Secret"
	ExtensionConfigTypeUrl = apiTypePrefix + "envoy.config.core.v3.TypedExtensionConfig"
	RuntimeTypeUrl         = apiTypePrefix + "envoy.service.runtime.v3.Runtime"
	HTTPConnManagerTypeUrl = apiTypePrefix + "envoy.extensions.filters.network.http_connection_manager.v3.HttpConnectionManager"
	// AnyType is used only by ADS
	AnyType = ""
)

var ResourceTypeToUrl = map[ResourceType]string{
	ListenerType:    ListenerTypeUrl,
	RouteConfigType: RouteTypeUrl,
	ClusterType:     ClusterTypeUrl,
	EndpointsType:   EndpointTypeUrl,
}

var ResourceUrlToType = map[string]ResourceType{
	ListenerTypeUrl: ListenerType,
	RouteTypeUrl:    RouteConfigType,
	ClusterTypeUrl:  ClusterType,
	EndpointTypeUrl: EndpointsType,
}

var ResourceTypeToName = map[ResourceType]string{
	ListenerType:    "Listener",
	RouteConfigType: "RouteConfig",
	ClusterType:     "Cluster",
	EndpointsType:   "ClusterLoadAssignment",
}
