package manager

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"sync"
	"time"

	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/xds/internal/xdsresource"
)

// xdsResourceManager manages all the xds resources in the cache and export Get function for resource retrieve.
// It uses client to fetch the resources from the control plane.
// It cleans the expired resource periodically.
type xdsResourceManager struct {
	// client communicates with the control plane
	client *xdsClient

	// cache stores all the resources
	cache map[xdsresource.ResourceType]map[string]xdsresource.Resource
	meta  map[xdsresource.ResourceType]map[string]*xdsresource.ResourceMeta
	// notifierMap maintains the channel for notifying resource update
	notifierMap map[xdsresource.ResourceType]map[string]*notifier
	mu          sync.Mutex

	// dumpPath is the path to dump the cached resource.
	dumpPath string
}

// notifier is used to notify the resource update along with error
type notifier struct {
	ch  chan struct{}
	err error
}

func (n *notifier) notify(err error) {
	n.err = err
	close(n.ch)
}

// NewXDSResourceManager creates a new xds resource manager
func NewXDSResourceManager(bootstrapConfig *BootstrapConfig) (*xdsResourceManager, error) {
	// load bootstrap config
	var err error
	if bootstrapConfig == nil {
		bootstrapConfig, err = newBootstrapConfig()
		if err != nil {
			return nil, err
		}
	}

	m := &xdsResourceManager{
		cache:       map[xdsresource.ResourceType]map[string]xdsresource.Resource{},
		meta:        make(map[xdsresource.ResourceType]map[string]*xdsresource.ResourceMeta),
		notifierMap: make(map[xdsresource.ResourceType]map[string]*notifier),
		mu:          sync.Mutex{},
		dumpPath:    defaultDumpPath,
	}
	// Initial xds client
	cli, err := newXdsClient(bootstrapConfig, m)
	if err != nil {
		return nil, err
	}

	m.client = cli

	// start the cache cleaner
	go m.cleaner()
	return m, nil
}

// getFromCache returns the resource from cache and update the access time in the meta
func (m *xdsResourceManager) getFromCache(rType xdsresource.ResourceType, rName string) (interface{}, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.cache[rType]; !ok {
		m.cache[rType] = make(map[string]xdsresource.Resource)
		return nil, false
	}

	if res, ok := m.cache[rType][rName]; !ok {
		return nil, false
	} else {
		// Record the timestamp
		now := time.Now()
		if _, ok := m.meta[rType]; !ok {
			m.meta[rType] = make(map[string]*xdsresource.ResourceMeta)
		}
		if _, ok := m.meta[rType][rName]; ok {
			m.meta[rType][rName].LastAccessTime = now
		} else {
			m.meta[rType][rName] = &xdsresource.ResourceMeta{
				LastAccessTime: now,
			}
		}
		return res, true
	}
}

// Get gets the specified resource from cache or from the control plane.
// If the resource is not in the cache, it will be fetched from the control plane via client.
// This will be a synchronous call. It uses the notifier to notify the resource update and return the resource.
func (m *xdsResourceManager) Get(ctx context.Context, rType xdsresource.ResourceType, rName string) (interface{}, error) {
	if _, ok := xdsresource.ResourceTypeToUrl[rType]; !ok {
		return nil, fmt.Errorf("[XDS ResourceManager] invalid resource type: %d", rType)
	}
	// Get from cache first
	res, ok := m.getFromCache(rType, rName)
	if ok {
		return res, nil
	}

	// Fetch resource via client and wait for the update
	m.mu.Lock()
	// Setup channel for this resource
	if _, ok := m.notifierMap[rType]; !ok {
		m.notifierMap[rType] = make(map[string]*notifier)
	}
	nf, ok := m.notifierMap[rType][rName]
	if !ok {
		// only send one request for this resource
		m.client.Watch(rType, rName)
		nf = &notifier{ch: make(chan struct{})}
		m.notifierMap[rType][rName] = nf
	}
	m.mu.Unlock()
	// Set fetch timeout
	// TODO: timeout should be specified in the config of xdsResourceManager
	timeout := defaultXDSFetchTimeout
	t := time.NewTimer(timeout)

	select {
	case <-nf.ch:
		// error in the notifier
		if nf.err != nil {
			return nil, fmt.Errorf("[XDS] manager, fetch %s resource[%s] failed, error=%s",
				xdsresource.ResourceTypeToName[rType], rName, nf.err.Error())
		}
		res, _ = m.getFromCache(rType, rName)
		return res, nil
	case <-t.C:
		// remove the notifier if timeout.
		m.mu.Lock()
		delete(m.notifierMap[rType], rName)
		m.mu.Unlock()
		return nil, fmt.Errorf("[XDS] manager, fetch %s resource[%s] failed, timeout %s",
			xdsresource.ResourceTypeToName[rType], rName, timeout)
	}
}

// cleaner cleans the expired cache periodically
func (m *xdsResourceManager) cleaner() {
	t := time.NewTicker(defaultCacheExpireTime)
	for {
		<-t.C
		m.mu.Lock()
		for rt := range m.meta {
			for rName, meta := range m.meta[rt] {
				if time.Since(meta.LastAccessTime) > defaultCacheExpireTime {
					delete(m.meta[rt], rName)
					delete(m.cache[rt], rName)
					m.client.RemoveWatch(rt, rName)
				}
			}
		}
		m.mu.Unlock()
		t.Reset(defaultRefreshInterval)
	}
}

// Dump dumps the cache to local file
// Dump when the cache is updated
func (m *xdsResourceManager) Dump() {
	m.mu.Lock()
	defer m.mu.Unlock()

	dumpResource := make(map[string]interface{})
	for rType, n := range xdsresource.ResourceTypeToName {
		if res, ok := m.cache[rType]; ok {
			dumpResource[n] = res
		}
	}

	path := m.dumpPath
	data, err := json.MarshalIndent(dumpResource, "", "    ")
	if err != nil {
		klog.Warnf("[XDS] manager, marshal xds resource failed when dumping, error=%s", err)
	}
	if err := ioutil.WriteFile(path, data, 0o644); err != nil {
		klog.Warnf("[XDS] manager, dump xds resource failed\n")
	}
}

// Close closes the xds manager
func (m *xdsResourceManager) Close() {
	// close xds client
	m.client.close()
}

// updateMeta updates the meta (version, updateTime) of the resource
func (m *xdsResourceManager) updateMeta(rType xdsresource.ResourceType, version string) {
	updateTime := time.Now()

	if _, ok := m.meta[rType]; !ok {
		m.meta[rType] = make(map[string]*xdsresource.ResourceMeta)
	}
	for name := range m.cache[rType] {
		if mt, ok := m.meta[rType][name]; ok {
			mt.UpdateTime = updateTime
			mt.Version = version
			continue
		}
		m.meta[rType][name] = &xdsresource.ResourceMeta{
			UpdateTime: updateTime,
			Version:    version,
		}
	}
	// remove the meta of the resource that is not in the cache
	for name := range m.meta[rType] {
		if _, ok := m.cache[rType][name]; !ok {
			delete(m.meta[rType], name)
		}
	}
}

//var (
//	ErrResourceNotFound = errors.New("resource not found in the latest xds response")
//)

/**
 	The following functions are invoked by xds client to update the cache.
	Logics of these functions are the same. Maybe refactor later using Generics.
*/

// UpdateListenerResource is invoked by client to update the cache
func (m *xdsResourceManager) UpdateListenerResource(up map[string]*xdsresource.ListenerResource, version string) {
	m.mu.Lock()

	for name, res := range up {
		if _, ok := m.cache[xdsresource.ListenerType]; !ok {
			m.cache[xdsresource.ListenerType] = make(map[string]xdsresource.Resource)
		}
		m.cache[xdsresource.ListenerType][name] = res
		if _, ok := m.notifierMap[xdsresource.ListenerType]; !ok {
			continue
		}
		if nf, exist := m.notifierMap[xdsresource.ListenerType][name]; exist {
			nf.notify(nil)
			delete(m.notifierMap[xdsresource.ListenerType], name)
		}
	}
	// remove all resources that are not in the new update
	for name := range m.cache[xdsresource.ListenerType] {
		if _, ok := up[name]; !ok {
			delete(m.cache[xdsresource.ListenerType], name)
		}
	}
	//// notify all watchers that the resource is not in the new update
	//for name, nf := range m.notifierMap[xdsresource.ListenerType] {
	//	nf.notify(ErrResourceNotFound)
	//	delete(m.notifierMap[xdsresource.ListenerType], name)
	//}
	// update meta
	m.updateMeta(xdsresource.ListenerType, version)
	m.mu.Unlock()
	m.Dump()
}

// UpdateRouteConfigResource is invoked by client to update the cache
func (m *xdsResourceManager) UpdateRouteConfigResource(up map[string]*xdsresource.RouteConfigResource, version string) {
	m.mu.Lock()

	for name, res := range up {
		if _, ok := m.cache[xdsresource.RouteConfigType]; !ok {
			m.cache[xdsresource.RouteConfigType] = make(map[string]xdsresource.Resource)
		}
		m.cache[xdsresource.RouteConfigType][name] = res
		if _, ok := m.notifierMap[xdsresource.RouteConfigType]; !ok {
			continue
		}
		if nf, exist := m.notifierMap[xdsresource.RouteConfigType][name]; exist {
			nf.notify(nil)
			delete(m.notifierMap[xdsresource.RouteConfigType], name)
		}
	}
	// remove all resources that are not in the update list
	for name := range m.cache[xdsresource.RouteConfigType] {
		if _, ok := up[name]; !ok {
			delete(m.cache[xdsresource.RouteConfigType], name)
		}
	}
	//// notify all watchers that the resource is not in the new update
	//for name, nf := range m.notifierMap[xdsresource.RouteConfigType] {
	//	nf.notify(ErrResourceNotFound)
	//	delete(m.notifierMap[xdsresource.RouteConfigType], name)
	//}
	// update meta
	m.updateMeta(xdsresource.RouteConfigType, version)
	m.mu.Unlock()
	m.Dump()
}

// UpdateClusterResource is invoked by client to update the cache
func (m *xdsResourceManager) UpdateClusterResource(up map[string]*xdsresource.ClusterResource, version string) {
	m.mu.Lock()

	for name, res := range up {
		if _, ok := m.cache[xdsresource.ClusterType]; !ok {
			m.cache[xdsresource.ClusterType] = make(map[string]xdsresource.Resource)
		}
		m.cache[xdsresource.ClusterType][name] = res
		if _, ok := m.notifierMap[xdsresource.ClusterType]; !ok {
			continue
		}
		if nf, exist := m.notifierMap[xdsresource.ClusterType][name]; exist {
			nf.notify(nil)
			delete(m.notifierMap[xdsresource.ClusterType], name)
		}
	}
	// remove all resources that are not in the update list
	for name := range m.cache[xdsresource.ClusterType] {
		if _, ok := up[name]; !ok {
			delete(m.cache[xdsresource.ClusterType], name)
		}
	}
	//// notify all watchers that the resource is not in the new update
	//for name, nf := range m.notifierMap[xdsresource.ClusterType] {
	//	nf.notify(ErrResourceNotFound)
	//	delete(m.notifierMap[xdsresource.ClusterType], name)
	//}
	// update meta
	m.updateMeta(xdsresource.ClusterType, version)
	m.mu.Unlock()
	m.Dump()
}

// UpdateEndpointsResource is invoked by client to update the cache
func (m *xdsResourceManager) UpdateEndpointsResource(up map[string]*xdsresource.EndpointsResource, version string) {
	m.mu.Lock()

	for name, res := range up {
		if _, ok := m.cache[xdsresource.EndpointsType]; !ok {
			m.cache[xdsresource.EndpointsType] = make(map[string]xdsresource.Resource)
		}
		m.cache[xdsresource.EndpointsType][name] = res
		if _, ok := m.notifierMap[xdsresource.EndpointsType]; !ok {
			continue
		}
		if nf, exist := m.notifierMap[xdsresource.EndpointsType][name]; exist {
			nf.notify(nil)
			delete(m.notifierMap[xdsresource.EndpointsType], name)
		}
	}
	// remove all resources that are not in the update list
	// TODO: endpoints cannot be removed in this case because Istio will perform incremental update of EDS.
	//for name := range m.cache[xdsresource.EndpointsType] {
	//	if _, ok := up[name]; !ok {
	//		delete(m.cache[xdsresource.EndpointsType], name)
	//	}
	//}

	//// notify all watchers that the resource is not in the new update
	//for name, nf := range m.notifierMap[xdsresource.EndpointsType] {
	//	nf.notify(ErrResourceNotFound)
	//	delete(m.notifierMap[xdsresource.EndpointsType], name)
	//}
	// update meta
	m.updateMeta(xdsresource.EndpointsType, version)
	m.mu.Unlock()
	m.Dump()
}
