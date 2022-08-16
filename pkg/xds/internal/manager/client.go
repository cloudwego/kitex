/*
 * Copyright 2021 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package manager

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/kitex/client"
	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/codes"
	discoveryv3 "github.com/cloudwego/kitex/pkg/xds/internal/api/kitex_gen/envoy/service/discovery/v3"
	"github.com/cloudwego/kitex/pkg/xds/internal/api/kitex_gen/envoy/service/discovery/v3/aggregateddiscoveryservice"
	"github.com/cloudwego/kitex/pkg/xds/internal/xdsresource"
	"google.golang.org/genproto/googleapis/rpc/status"
)

type (
	ADSClient = aggregateddiscoveryservice.Client
	ADSStream = aggregateddiscoveryservice.AggregatedDiscoveryService_StreamAggregatedResourcesClient
)

// xdsClient communicates with the control plane to perform xds resource discovery.
// It maintains the connection and all the resources being watched.
// It processes the responses from the xds server and update the resources to the cache in xdsResourceManager.
type xdsClient struct {
	// config is the bootstrap config read from the config file.
	config *BootstrapConfig

	// watchedResource is the map of resources that are watched by the client.
	// every discovery request will contain all the resources of one type in the map.
	watchedResource map[xdsresource.ResourceType]map[string]bool

	// cipResolver is used to resolve the clusterIP, using NDS.
	// listenerName (we use fqdn for listener name) to clusterIP.
	// "kitex-server.default.svc.cluster.local" -> "10.0.0.1"
	cipResolver *ndsResolver

	// versionMap stores the versions of different resource type.
	versionMap map[xdsresource.ResourceType]string
	// nonceMap stores the nonce of the recent response.
	nonceMap map[xdsresource.ResourceType]string

	// adsClient is a kitex client using grpc protocol that can communicate with xds server.
	adsClient        ADSClient
	adsStream        ADSStream
	streamClientLock sync.Mutex

	// resourceUpdater is used to update the resource update to the cache.
	resourceUpdater *xdsResourceManager

	// refreshInterval is the interval of refreshing the resources.
	refreshInterval time.Duration

	// channel for stop
	closeCh chan struct{}

	mu sync.RWMutex
}

func newNdsResolver() *ndsResolver {
	return &ndsResolver{
		mu:            sync.Mutex{},
		lookupTable:   make(map[string][]string),
		initRequestCh: make(chan struct{}),
	}
}

// ndsResolver is used to resolve the clusterIP, using NDS.
type ndsResolver struct {
	lookupTable   map[string][]string
	mu            sync.Mutex
	initRequestCh chan struct{}
}

// lookupHost returns the clusterIP of the given hostname.
func (r *ndsResolver) lookupHost(host string) ([]string, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	ips, ok := r.lookupTable[host]
	return ips, ok
}

// updateResource updates the lookup table based on the NDS response.
func (r *ndsResolver) updateLookupTable(up map[string][]string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.initRequestCh != nil {
		close(r.initRequestCh)
	}
	r.lookupTable = up
}

// newADSClient constructs a new stream client that communicates with the xds server
func newADSClient(addr string) (ADSClient, error) {
	cli, err := aggregateddiscoveryservice.NewClient("xds_servers",
		client.WithHostPorts(addr),
	)
	if err != nil {
		return nil, fmt.Errorf("[XDS] client: construct ads client failed, %s", err.Error())
	}
	return cli, nil
}

// newXdsClient constructs a new xdsClient, which is used to get xds resources from the xds server.
func newXdsClient(bCfg *BootstrapConfig, updater *xdsResourceManager) (*xdsClient, error) {
	// build stream client that communicates with the xds server
	ac, err := newADSClient(bCfg.xdsSvrCfg.SvrAddr)
	if err != nil {
		return nil, fmt.Errorf("[XDS] client: construct stream client failed, %s", err.Error())
	}

	cli := &xdsClient{
		config:          bCfg,
		adsClient:       ac,
		watchedResource: make(map[xdsresource.ResourceType]map[string]bool),
		cipResolver:     newNdsResolver(),
		versionMap:      make(map[xdsresource.ResourceType]string),
		nonceMap:        make(map[xdsresource.ResourceType]string),
		resourceUpdater: updater,
		refreshInterval: defaultRefreshInterval,
		closeCh:         make(chan struct{}),
	}
	cli.run()
	return cli, nil
}

// Watch adds a resource to the watch map and send a discovery request.
func (c *xdsClient) Watch(rType xdsresource.ResourceType, rName string) {
	c.mu.Lock()
	// New resource type
	if r := c.watchedResource[rType]; r == nil {
		c.watchedResource[rType] = make(map[string]bool)
	}
	// subscribe new resource
	c.watchedResource[rType][rName] = true
	c.mu.Unlock()
	// send request for this resource
	c.sendRequest(rType, false, "")
}

// RemoveWatch removes a resource from the watch map.
func (c *xdsClient) RemoveWatch(rType xdsresource.ResourceType, rName string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, ok := c.watchedResource[rType]; !ok {
		return
	}
	// remove this resource
	delete(c.watchedResource[rType], rName)
}

// refresh all resources
func (c *xdsClient) refresh() {
	for rt := range c.watchedResource {
		c.sendRequest(rt, false, "")
	}
}

// sender refresh the resources periodically
func (c *xdsClient) sender() {
	timer := time.NewTicker(c.refreshInterval)

	// sender
	defer func() {
		if err := recover(); err != nil {
			klog.Errorf("[XDS] client: run sender panic, error=%s", err)
		}
	}()

	for {
		select {
		case <-c.closeCh:
			klog.Infof("[XDS] client: stop ads client sender")
			return
		case <-timer.C:
			// c.refresh()
			timer.Reset(c.refreshInterval)
		}
	}
}

// receiver receives and handle response from the xds server.
// xds server may proactively push the update.
func (c *xdsClient) receiver() {
	// receiver
	defer func() {
		if err := recover(); err != nil {
			klog.Errorf("[XDS] client: run receiver panic, error=%s", err)
		}
	}()

	for {
		select {
		case <-c.closeCh:
			klog.Infof("[XDS] client: stop ads client receiver")
			return
		default:
		}
		resp, err := c.recv()
		if err != nil {
			klog.Errorf("[XDS] client, receive failed, error=%s", err)
			c.reconnect()
			continue
		}
		err = c.handleResponse(resp)
		if err != nil {
			klog.Errorf("[XDS] client, handle response failed, error=%s", err)
		}
	}
}

func (c *xdsClient) run() {
	// two goroutines for sender and receiver
	go c.sender()
	go c.receiver()
	c.warmup()
}

// close the xdsClient
func (c *xdsClient) close() {
	close(c.closeCh)
	if c.adsStream != nil {
		c.adsStream.Close()
	}
}

// getStreamClient returns the adsClient of xdsClient
func (c *xdsClient) getStreamClient() (ADSStream, error) {
	c.streamClientLock.Lock()
	defer c.streamClientLock.Unlock()
	// get stream client
	if c.adsStream != nil {
		return c.adsStream, nil
	}
	// connect
	as, err := c.connect()
	if err != nil {
		return nil, err
	}
	c.adsStream = as
	return c.adsStream, nil
}

// connect construct a new stream that connects to the xds server
func (c *xdsClient) connect() (ADSStream, error) {
	as, err := c.adsClient.StreamAggregatedResources(context.Background())
	if err != nil {
		return nil, err
	}
	return as, nil
}

// reconnect construct a new stream and send all the watched resources
func (c *xdsClient) reconnect() error {
	c.streamClientLock.Lock()
	defer c.streamClientLock.Unlock()
	// close old stream
	if c.adsStream != nil {
		c.adsStream.Close()
	}
	// create new stream
	as, err := c.connect()
	if err != nil {
		return err
	}
	c.adsStream = as

	// reset the version map, nonce map and send new requests when reconnect
	c.mu.Lock()
	c.versionMap = make(map[xdsresource.ResourceType]string)
	c.nonceMap = make(map[xdsresource.ResourceType]string)
	c.mu.Unlock()
	for rt := range c.watchedResource {
		req := c.prepareRequest(rt, false, "")
		_ = c.adsStream.Send(req)
	}
	return nil
}

// sendRequest prepares the requests and sends to the xds server
func (c *xdsClient) sendRequest(rType xdsresource.ResourceType, ack bool, errMsg string) {
	// prepare request
	req := c.prepareRequest(rType, ack, errMsg)
	if req == nil {
		return
	}
	// get stream client
	sc, err := c.getStreamClient()
	if err != nil {
		klog.Errorf("[XDS] client, get stream client failed, error=%s", err)
		return
	}
	err = sc.Send(req)
	if err != nil {
		klog.Errorf("[XDS] client, send failed, error=%s", err)
	}
}

// recv uses stream client to receive the response from the xds server
func (c *xdsClient) recv() (resp *discoveryv3.DiscoveryResponse, err error) {
	sc, err := c.getStreamClient()
	if err != nil {
		return nil, err
	}
	resp, err = sc.Recv()
	return resp, err
}

// updateAndACK update versionMap, nonceMap and send ack to xds server
func (c *xdsClient) updateAndACK(rType xdsresource.ResourceType, nonce, version string, err error) {
	// update nonce and version
	// update and ACK only if error is nil, or NACK should be sent
	var errMsg string
	c.mu.Lock()
	if err == nil {
		c.versionMap[rType] = version
	} else {
		errMsg = err.Error()
	}
	c.nonceMap[rType] = nonce
	c.mu.Unlock()
	c.sendRequest(rType, true, errMsg)
}

// warmup sends the requests (NDS) to the xds server and waits for the response to set the lookup table.
func (c *xdsClient) warmup() {
	// watch the NameTable when init the xds client
	c.Watch(xdsresource.NameTableType, "")
	<-c.cipResolver.initRequestCh
	klog.Infof("[XDS] client, warmup done")
	c.cipResolver.initRequestCh = nil
	// TODO: maybe need to watch the listener
}

// getListenerName returns the listener name in this format: ${clusterIP}_${port}
// lookup the clusterIP using the cipResolver and return the listenerName
func (c *xdsClient) getListenerName(rName string) (string, error) {
	tmp := strings.Split(rName, ":")
	if len(tmp) < 2 {
		return "", fmt.Errorf("invalid listener name: %s", rName)
	}
	addr, port := tmp[0], tmp[1]
	cip, ok := c.cipResolver.lookupHost(addr)
	if ok && len(cip) > 0 {
		clusterIPPort := cip[0] + "_" + port
		return clusterIPPort, nil
	}
	return "", fmt.Errorf("failed to convert listener name for %s", rName)
}

// handleLDS handles the lds response
func (c *xdsClient) handleLDS(resp *discoveryv3.DiscoveryResponse) error {
	res, err := xdsresource.UnmarshalLDS(resp.GetResources())
	c.updateAndACK(xdsresource.ListenerType, resp.GetNonce(), resp.GetVersionInfo(), err)
	if err != nil {
		return err
	}

	// returned listener name is in the format of ${clusterIP}_${port}
	// which should be converted into to the listener name, in the form of ${fqdn}_${port}, watched by the xds client.
	// we need to filter the response.
	c.mu.RLock()
	defer c.mu.RUnlock()
	filteredRes := make(map[string]*xdsresource.ListenerResource)
	for n := range c.watchedResource[xdsresource.ListenerType] {
		ln, err := c.getListenerName(n)
		if err != nil || ln == "" {
			// klog.Warnf("[XDS] client, handleLDS failed, error=%s", err)
			continue
		}
		if lis, ok := res[ln]; ok {
			filteredRes[n] = lis
		}
	}
	// update to cache
	c.resourceUpdater.UpdateListenerResource(filteredRes, resp.GetVersionInfo())
	return nil
}

// handleRDS handles the rds response
func (c *xdsClient) handleRDS(resp *discoveryv3.DiscoveryResponse) error {
	res, err := xdsresource.UnmarshalRDS(resp.GetResources())
	c.updateAndACK(xdsresource.RouteConfigType, resp.GetNonce(), resp.GetVersionInfo(), err)
	if err != nil {
		return err
	}

	// filter the resources that are not in the watched list
	c.mu.RLock()
	defer c.mu.RUnlock()
	for name := range res {
		// only accept the routeConfig that is subscribed
		if _, ok := c.watchedResource[xdsresource.RouteConfigType][name]; !ok {
			delete(res, name)
		}
	}
	// update to cache
	c.resourceUpdater.UpdateRouteConfigResource(res, resp.GetVersionInfo())
	return nil
}

// handleCDS handles the cds response
func (c *xdsClient) handleCDS(resp *discoveryv3.DiscoveryResponse) error {
	res, err := xdsresource.UnmarshalCDS(resp.GetResources())
	c.updateAndACK(xdsresource.ClusterType, resp.GetNonce(), resp.GetVersionInfo(), err)
	if err != nil {
		return fmt.Errorf("handle cluster failed: %s", err)
	}

	// filter the resources that are not in the watched list
	c.mu.RLock()
	defer c.mu.RUnlock()
	for name := range res {
		if _, ok := c.watchedResource[xdsresource.ClusterType][name]; !ok {
			delete(res, name)
		}
	}
	// update to cache
	c.resourceUpdater.UpdateClusterResource(res, resp.GetVersionInfo())
	return nil
}

// handleEDS handles the eds response
func (c *xdsClient) handleEDS(resp *discoveryv3.DiscoveryResponse) error {
	res, err := xdsresource.UnmarshalEDS(resp.GetResources())
	c.updateAndACK(xdsresource.EndpointsType, resp.GetNonce(), resp.GetVersionInfo(), err)
	if err != nil {
		return fmt.Errorf("handle endpoint failed: %s", err)
	}

	// filter the resources that are not in the watched list
	c.mu.RLock()
	defer c.mu.RUnlock()
	for name := range res {
		if _, ok := c.watchedResource[xdsresource.EndpointsType][name]; !ok {
			delete(res, name)
		}
	}
	// update to cache
	c.resourceUpdater.UpdateEndpointsResource(res, resp.GetVersionInfo())
	return nil
}

func (c *xdsClient) handleNDS(resp *discoveryv3.DiscoveryResponse) error {
	nt, err := xdsresource.UnmarshalNDS(resp.GetResources())
	c.updateAndACK(xdsresource.NameTableType, resp.GetNonce(), resp.GetVersionInfo(), err)
	if err != nil {
		return err
	}
	c.cipResolver.updateLookupTable(nt.NameTable)
	return err
}

// handleResponse handles the response from xDS server
func (c *xdsClient) handleResponse(msg interface{}) error {
	if msg == nil {
		return nil
	}
	// check the type of response
	resp, ok := msg.(*discoveryv3.DiscoveryResponse)
	if !ok {
		return fmt.Errorf("invalid discovery response")
	}

	url := resp.GetTypeUrl()
	rType, ok := xdsresource.ResourceUrlToType[url]
	if !ok {
		klog.Warnf("unknown type of resource, url: %s", url)
		return nil
	}

	// check if this resource type is watched
	c.mu.RLock()
	if _, ok := c.watchedResource[rType]; !ok {
		c.mu.RUnlock()
		return nil
	}
	c.mu.RUnlock()

	/*
		handle different resources:
		unmarshal resources and ack, update to cache if no error
	*/
	var err error
	switch rType {
	case xdsresource.ListenerType:
		err = c.handleLDS(resp)
	case xdsresource.RouteConfigType:
		err = c.handleRDS(resp)
	case xdsresource.ClusterType:
		err = c.handleCDS(resp)
	case xdsresource.EndpointsType:
		err = c.handleEDS(resp)
	case xdsresource.NameTableType:
		err = c.handleNDS(resp)
	}
	return err
}

// prepareRequest prepares a new request for the specified resource type
// ResourceNames should include all the subscribed resources of the specified type
func (c *xdsClient) prepareRequest(rType xdsresource.ResourceType, ack bool, errMsg string) *discoveryv3.DiscoveryRequest {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var (
		version string
		nonce   string
		rNames  []string
	)
	res, ok := c.watchedResource[rType]
	if !ok || len(res) == 0 {
		return nil
	}
	// prepare resource name
	rNames = make([]string, 0, len(res))
	for name := range res {
		rNames = append(rNames, name)
	}
	// prepare version and nonce
	version = c.versionMap[rType]
	nonce = c.nonceMap[rType]
	req := &discoveryv3.DiscoveryRequest{
		VersionInfo:   version,
		Node:          c.config.node,
		TypeUrl:       xdsresource.ResourceTypeToUrl[rType],
		ResourceNames: rNames,
		ResponseNonce: nonce,
	}

	// NACK with error message
	if ack && errMsg != "" {
		req.ErrorDetail = &status.Status{
			Code: int32(codes.InvalidArgument), Message: errMsg,
		}
	}
	return req
}
