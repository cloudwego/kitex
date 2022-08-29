package remoteconfig

import (
	"sync"
)

var _ Configurator = &configurator{}

type refCountable interface {
	Incr()
	Decr()
}

type configurator struct {
	lock     sync.RWMutex
	notifies []ChangeNotify
	refCountable
}

func (c *configurator) RegisterChangeNotify(notify ChangeNotify) {
	c.lock.Lock()
	c.notifies = append(c.notifies, notify)
	c.lock.Unlock()
}

func (c *configurator) Close() error {
	c.lock.Lock()
	c.notifies = nil
	c.lock.Unlock()
	if c.refCountable != nil {
		c.Decr()
	}
	return nil
}

func (c *configurator) OnDataChange(key string, oldData, newData interface{}) {
	c.lock.RLock()
	for _, notify := range c.notifies {
		notify(key, oldData, newData)
	}
	c.lock.RUnlock()
}
