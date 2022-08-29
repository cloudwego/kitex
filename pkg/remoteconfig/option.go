package remoteconfig

import (
	"time"
)

var defaultRefreshInterval = 10 * time.Second

// ChangeHandler is the callback function after data changed.
type ChangeHandler func() error

// DataComparer compare Config, return true if equal.
type DataComparer func(old, new map[string]Config) bool

// ManagerOption set values of ManagerOptions.
type ManagerOption func(*ManagerOptions)

// ManagerOptions contains settings of ConfigurationManager.
type ManagerOptions struct {
	changeHandler func() error
}

func newManagerOptions(opt ...ManagerOption) ManagerOptions {
	opts := ManagerOptions{}
	for _, o := range opt {
		o(&opts)
	}
	return opts
}

// WithChangeHandler set ChangeHandler of ConfigurationManager.
func WithChangeHandler(handler ChangeHandler) ManagerOption {
	return func(o *ManagerOptions) {
		o.changeHandler = handler
	}
}

// ProviderOption set values of ProviderOptions.
type ProviderOption func(*ProviderOptions)

// ProviderOptions contains settings of Provider.
type ProviderOptions struct {
	refreshInterval time.Duration
	dataComparer    DataComparer
}

func newProviderOptions(opt ...ProviderOption) ProviderOptions {
	opts := ProviderOptions{
		refreshInterval: defaultRefreshInterval,
	}
	for _, o := range opt {
		o(&opts)
	}
	return opts
}

// WithRefreshInterval set refreshInterval of Provider.
func WithRefreshInterval(interval time.Duration) ProviderOption {
	return func(o *ProviderOptions) {
		o.refreshInterval = interval
	}
}

// WithDataComparer set dataComparer of Provider.
func WithDataComparer(comparer DataComparer) ProviderOption {
	return func(o *ProviderOptions) {
		o.dataComparer = comparer
	}
}

// ConfiguratorOption set values of ConfiguratorOptions.
type ConfiguratorOption func(*ConfiguratorOptions)

// InitHandler is the callback function after register an new Configurator.
type InitHandler func(configs map[string]Config) error

// ConfiguratorOptions contains settings of Configurator.
type ConfiguratorOptions struct {
	changeNotify ChangeNotify
	initHandler  InitHandler
}

func newConfiguratorOptions(opt ...ConfiguratorOption) ConfiguratorOptions {
	opts := ConfiguratorOptions{}
	for _, o := range opt {
		o(&opts)
	}
	return opts
}

// WithChangeNotify set changeNotify when new Configurator.
func WithChangeNotify(changeNotify ChangeNotify) ConfiguratorOption {
	return func(o *ConfiguratorOptions) {
		o.changeNotify = changeNotify
	}
}

// WithInitHandler set initHandler after register an new Configurator.
func WithInitHandler(initHandler InitHandler) ConfiguratorOption {
	return func(o *ConfiguratorOptions) {
		o.initHandler = initHandler
	}
}
