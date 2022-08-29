package remoteconfig

import (
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/kitex/pkg/klog"
)

var _ Provider = &provider{}

type provider struct {
	options          ProviderOptions
	data             map[string]Config
	lock             sync.RWMutex
	serviceMetas     *serviceMetas
	configCenter     DynamicConfigCenter
	fetcher          FetchFunc
	dataChangeSignal chan struct{}
	notifyRefresh    chan *notifyRefresh
}

type notifyRefresh struct {
	done chan struct{}
}

func newNotifyRefresh() *notifyRefresh {
	return &notifyRefresh{
		done: make(chan struct{}, 1),
	}
}

type serviceMetas struct {
	callers sync.Map
	callees sync.Map
}

func genServiceMetaID(meta ServiceMeta) string {
	id := meta.Name() + strconv.Itoa(int(meta.Type()))
	if tags := meta.Tags(); len(tags) > 0 {
		keys := make([]string, 0, len(tags))
		for k := range tags {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			id += k
			id += tags[k]
		}
	}
	return id
}

func (metas *serviceMetas) get(category ServiceType) []ServiceMeta {
	var ret []ServiceMeta
	switch category {
	case TypeCaller:
		metas.callers.Range(func(key, value interface{}) bool {
			ret = append(ret, value.(ServiceMeta))
			return true
		})
		return ret
	case TypeCallee:
		metas.callees.Range(func(key, value interface{}) bool {
			ret = append(ret, value.(ServiceMeta))
			return true
		})
		return ret
	default:
		return nil
	}
}

func (metas *serviceMetas) add(meta ServiceMeta) bool {
	var (
		existed = false
		metaID  = genServiceMetaID(meta)
	)
	switch meta.Type() {
	case TypeCaller:
		_, existed = metas.callers.LoadOrStore(metaID, meta)
	case TypeCallee:
		_, existed = metas.callees.LoadOrStore(metaID, meta)
	}
	return !existed
}

func (metas *serviceMetas) delete(meta ServiceMeta, condition func(key string) bool) bool {
	deleted := false
	switch meta.Type() {
	case TypeCaller:
		metas.callers.Range(func(k, v interface{}) bool {
			if condition(k.(string)) {
				metas.callers.Delete(k)
				deleted = true
			}
			return true
		})
	case TypeCallee:
		metas.callees.Range(func(k, v interface{}) bool {
			if condition(k.(string)) {
				metas.callees.Delete(k)
				deleted = true
			}
			return true
		})
	}
	return deleted
}

// NewProvider return the provider which provide remoteconfig data.
func NewProvider(configCenter DynamicConfigCenter, fetchFunc FetchFunc, opts ...ProviderOption) Provider {
	options := newProviderOptions(opts...)
	p := &provider{
		options: options,
		serviceMetas: &serviceMetas{
			callers: sync.Map{},
			callees: sync.Map{},
		},
		dataChangeSignal: make(chan struct{}, 1),
		notifyRefresh:    make(chan *notifyRefresh, 1),
		fetcher:          fetchFunc,
		configCenter:     configCenter,
	}
	go p.refresh()
	return p
}

func (p *provider) Put(meta ServiceMeta) {
	if meta == nil {
		return
	}
	if ok := p.serviceMetas.add(meta); ok {
		notify := newNotifyRefresh()
		p.notifyRefresh <- notify
		// wait refresh finished avoid cannot get correct data at initialization phase
		<-notify.done
	}
}

func (p *provider) Remove(meta ServiceMeta) {
	if meta == nil {
		return
	}
	metaID := genServiceMetaID(meta)
	condition := func(key string) bool {
		return strings.HasPrefix(key, metaID)
	}
	if ok := p.serviceMetas.delete(meta, condition); ok {
		p.notifyRefresh <- newNotifyRefresh()
	}
}

func (p *provider) refresh() {
	refresher := func() error {
		var (
			err              error
			oldData, newData map[string]Config
			callers          = p.serviceMetas.get(TypeCaller)
			callees          = p.serviceMetas.get(TypeCallee)
		)

		defer func() {
			if comparer := p.options.dataComparer; comparer != nil {
				if !comparer(oldData, newData) {
					klog.Debugf("KITEX: remote remoteconfig changed, prepare to reload.")
					p.dataChangeSignal <- struct{}{}
				}
			} else {
				// comparer is nil, always notify data changed signal
				p.dataChangeSignal <- struct{}{}
			}
		}()

		newData, err = p.fetcher(p.configCenter, callers, callees)
		if err != nil {
			return err
		}
		if newData == nil {
			// should never return nil except:
			// 1. not modified
			// 2. remoteconfig center is nil
			return nil
		}

		p.lock.Lock()
		oldData = p.data
		p.data = newData
		p.lock.Unlock()

		return nil
	}

	ticker := time.NewTicker(p.options.refreshInterval)
	defer ticker.Stop()
	for {
		select {
		case notify := <-p.notifyRefresh:
			if err := refresher(); err != nil {
				klog.Warnf("KITEX: refresh remote remoteconfig failed, error=%s", err.Error())
			}
			notify.done <- struct{}{}
		case <-ticker.C:
			if err := refresher(); err != nil {
				klog.Warnf("KITEX: refresh remote remoteconfig failed, error=%s", err.Error())
			}
		}
	}
}

func (p *provider) DataChangeTrigger() chan struct{} {
	return p.dataChangeSignal
}

func (p *provider) Get() map[string]Config {
	p.lock.RLock()
	defer p.lock.RUnlock()
	return p.data
}
