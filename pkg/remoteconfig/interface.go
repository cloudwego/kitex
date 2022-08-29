// Package remoteconfig define interface about remote configs.
package remoteconfig

// Config is configuration been stored in dynamic remoteconfig center.
type Config interface {
	Equal(Config) bool
}

// ConfigurationManager provides access to get remoteconfig, and manages configurator's lifecycle.
type ConfigurationManager interface {
	Dumper
	Get(key string) interface{}
	RegisterCaller(serviceName string, tags map[string]string, opts ...ConfiguratorOption) Configurator
	RegisterCallee(serviceName string, tags map[string]string, opts ...ConfiguratorOption) Configurator
}

// Dumper dump cached data.
type Dumper interface {
	Dump() interface{}
}

// ChangeNotify is the callback function type for cache.
// It will be invoked when the cached item changes.
type ChangeNotify func(key string, oldData, newData interface{})

// Configurator binding on client or server, when client/server close would be destroyed.
type Configurator interface {
	RegisterChangeNotify(notify ChangeNotify)
	OnDataChange(key string, oldData, newData interface{})
	Close() error
}

// Provider provides remoteconfig data.
type Provider interface {
	ServiceMetaManager
	Get() map[string]Config
	DataChangeTrigger() chan struct{}
}

// ServiceMetaManager manage ServiceMeta.
type ServiceMetaManager interface {
	Put(meta ServiceMeta)
	Remove(meta ServiceMeta)
}

// ServiceType is the type of service: caller or callee.
type ServiceType int8

// TypeCaller and TypeCallee representative type of service.
const (
	TypeCaller ServiceType = iota
	TypeCallee
)

// ServiceMeta describe metas about service.
type ServiceMeta interface {
	Name() string
	Type() ServiceType
	Tags() map[string]string // cluster, method, etc
}

// DynamicConfigCenter manage relationship and lifecycle of remote configurations.
type DynamicConfigCenter interface {
	Resolver
	GetCallerCfg(serviceMeta ServiceMeta, category string) (interface{}, error)
	GetCalleeCfg(serviceMeta ServiceMeta, category string) (interface{}, error)
	GetGlobalCfg(category string) (interface{}, error)
}

// Resolver fetches endpoints of DynamicConfigCenter.
type Resolver interface {
	ResolveNow() error
	GetEndpoint() string
}

// FetchFunc define the func which fetch remote configuration from dynamic remoteconfig center.
type FetchFunc func(configCenter DynamicConfigCenter, callers, callees []ServiceMeta) (map[string]Config, error)
