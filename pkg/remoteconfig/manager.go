package remoteconfig

import (
	"sync"
	"sync/atomic"

	"github.com/cloudwego/kitex/pkg/klog"
)

var (
	_ ConfigurationManager = &configurationManager{}
	_ ServiceMeta          = &serviceMeta{}
)

type closeGroupFunc func() error

type configuratorGroup struct {
	count         int32
	lock          sync.RWMutex
	configurators []Configurator
	onClose       closeGroupFunc
}

func (g *configuratorGroup) Incr() {
	atomic.AddInt32(&g.count, 1)
}

func (g *configuratorGroup) Decr() {
	atomic.AddInt32(&g.count, -1)
	if atomic.LoadInt32(&g.count) <= 0 && g.onClose != nil {
		_ = g.onClose()
	}
}

func (g *configuratorGroup) add() *configurator {
	c := &configurator{
		refCountable: g,
	}

	g.lock.Lock()
	g.configurators = append(g.configurators, c)
	g.lock.Unlock()

	c.Incr()
	return c
}

type configurationManager struct {
	options       ManagerOptions
	provider      Provider
	defaultConfig Config
	callerGroups  sync.Map // map[string]*configuratorGroup
	calleeGroups  sync.Map // map[string]*configuratorGroup
	data          sync.Map // map[key]*Config
}

type serviceMeta struct {
	name     string
	category ServiceType
	tags     map[string]string
}

func (s *serviceMeta) Name() string {
	return s.name
}

func (s *serviceMeta) Type() ServiceType {
	return s.category
}

func (s *serviceMeta) Tags() map[string]string {
	return s.tags
}

// NewConfiguratorManager new an ConfigurationManager which manage configuration.
func NewConfiguratorManager(provider Provider, defaultConfig Config, opt ...ManagerOption) ConfigurationManager {
	options := newManagerOptions(opt...)
	manager := &configurationManager{
		options:       options,
		provider:      provider,
		defaultConfig: defaultConfig,
		data:          sync.Map{},
		callerGroups:  sync.Map{},
		calleeGroups:  sync.Map{},
	}
	go manager.onDataChange()

	return manager
}

func (m *configurationManager) newConfiguratorGroup(onClose closeGroupFunc) *configuratorGroup {
	g := &configuratorGroup{
		configurators: []Configurator{},
		onClose:       onClose,
	}
	atomic.StoreInt32(&g.count, 1)
	return g
}

func (m *configurationManager) onDataChange() {
	for range m.provider.DataChangeTrigger() {
		if err := m.reloadData(); err != nil {
			klog.Warnf("KITEX: Reload remote remoteconfig failed when data changed, error=%s", err.Error())
			continue
		}

		if changeHandler := m.options.changeHandler; changeHandler != nil {
			if err := changeHandler(); err != nil {
				klog.Warnf("KITEX: Exec handler failed after configurationManager reload remote remoteconfig, error=%s", err.Error())
			}
		}
	}
}

func (m *configurationManager) Get(key string) interface{} {
	if val, ok := m.data.Load(key); ok {
		return val
	}
	return m.defaultConfig
}

func (m *configurationManager) RegisterCaller(serviceName string, tags map[string]string, opts ...ConfiguratorOption) Configurator {
	val, _ := m.callerGroups.LoadOrStore(serviceName, m.newConfiguratorGroup(func() error {
		m.callerGroups.Delete(serviceName)
		m.provider.Remove(&serviceMeta{
			name:     serviceName,
			category: TypeCaller,
		})
		return nil
	}))
	group := val.(*configuratorGroup)

	c := group.add()
	options := newConfiguratorOptions(opts...)
	if changeNotify := options.changeNotify; changeNotify != nil {
		c.RegisterChangeNotify(changeNotify)
	}
	defer func() {
		if handler := options.initHandler; handler != nil {
			if err := handler(m.provider.Get()); err != nil {
				klog.Warnf("KITEX: Exec handler failed after register caller, error=%s", err.Error())
			}
		}
	}()

	// maybe notify provider to get latest data
	m.provider.Put(&serviceMeta{
		name:     serviceName,
		category: TypeCaller,
		tags:     tags,
	})

	return c
}

func (m *configurationManager) RegisterCallee(serviceName string, tags map[string]string, opts ...ConfiguratorOption) Configurator {
	val, _ := m.calleeGroups.LoadOrStore(serviceName, m.newConfiguratorGroup(func() error {
		m.calleeGroups.Delete(serviceName)
		m.provider.Remove(&serviceMeta{
			name:     serviceName,
			category: TypeCallee,
		})
		return nil
	}))
	group := val.(*configuratorGroup)

	c := group.add()
	options := newConfiguratorOptions(opts...)
	if changeNotify := options.changeNotify; changeNotify != nil {
		c.RegisterChangeNotify(changeNotify)
	}
	defer func() {
		if handler := options.initHandler; handler != nil {
			if err := handler(m.provider.Get()); err != nil {
				klog.Warnf("KITEX: Exec handler failed after register callee, error=%s", err.Error())
			}
		}
	}()

	// maybe notify provider to get latest data
	m.provider.Put(&serviceMeta{
		name:     serviceName,
		category: TypeCallee,
		tags:     tags,
	})

	return c
}

func (m *configurationManager) Dump() interface{} {
	data := make(map[string]interface{})
	m.data.Range(func(key, val interface{}) bool {
		k := key.(string)
		data[k] = val
		return true
	})
	return data
}

func (m *configurationManager) reloadData() error {
	newData := m.provider.Get()
	if newData != nil {
		m.data.Range(func(key, val interface{}) bool {
			k := key.(string)
			if newVal, ok := newData[k]; ok {
				if !newVal.Equal(val.(Config)) {
					m.notifyChange(k, val, newVal) // change
				}
				m.data.Store(k, newVal)
			} else {
				m.notifyChange(k, val, nil) // delete
				m.data.Delete(k)
			}
			return true
		})
		for newKey, newVal := range newData {
			if _, existed := m.data.Load(newKey); !existed {
				m.notifyChange(newKey, nil, newVal) // add
				m.data.Store(newKey, newVal)
			}
		}
	}
	return nil
}

func (m *configurationManager) notifyChange(key string, oldVal, newVal interface{}) {
	m.callerGroups.Range(func(k, v interface{}) bool {
		cGroup := v.(*configuratorGroup)
		cGroup.lock.RLock()
		for _, c := range cGroup.configurators {
			c.OnDataChange(key, oldVal, newVal)
		}
		cGroup.lock.RUnlock()
		return true
	})
	m.calleeGroups.Range(func(k, v interface{}) bool {
		cGroup := v.(*configuratorGroup)
		cGroup.lock.RLock()
		for _, c := range cGroup.configurators {
			c.OnDataChange(key, oldVal, newVal)
		}
		cGroup.lock.RUnlock()
		return true
	})
}
