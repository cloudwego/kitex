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

package remoteinfo

import (
	"errors"
	"net"
	"sync"

	"github.com/cloudwego/kitex/internal"
	"github.com/cloudwego/kitex/pkg/discovery"
	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
)

// RemoteInfo implements a rpcinfo.EndpointInfo with mutable address and connection.
// It is typically used to represent server info on client side or client info on server side.
type RemoteInfo interface {
	rpcinfo.EndpointInfo
	SetServiceName(name string)
	SetTag(key, value string) error
	ForceSetTag(key, value string)

	// SetTagLock freezes a key of the tags and refuses further modification on its value.
	SetTagLock(key string)
	GetInstance() discovery.Instance
	SetInstance(ins discovery.Instance)

	// SetRemoteAddr tries to set the network address of the discovery.Instance hold by RemoteInfo.
	// The result indicates whether the modification is successful.
	SetRemoteAddr(addr net.Addr) (ok bool)
	CopyFrom(from RemoteInfo)
	ImmutableView() rpcinfo.EndpointInfo
}

// RemoteAddrSetter is used to set remote addr.
type RemoteAddrSetter interface {
	SetRemoteAddr(addr net.Addr) (ok bool)
}

var (
	_              RemoteInfo = &remoteInfo{}
	remoteInfoPool sync.Pool
)

type remoteInfo struct {
	sync.RWMutex
	serviceName string
	method      string
	tags        map[string]string

	instance discovery.Instance
	tagLocks map[string]struct{}
}

func init() {
	remoteInfoPool.New = newRemoteInfo
}

func newRemoteInfo() interface{} {
	return &remoteInfo{
		tags:     make(map[string]string),
		tagLocks: make(map[string]struct{}),
	}
}

// GetInstance implements the RemoteInfo interface.
func (ri *remoteInfo) GetInstance() (ins discovery.Instance) {
	ri.RLock()
	ins = ri.instance
	ri.RUnlock()
	return
}

// SetInstance implements the RemoteInfo interface.
func (ri *remoteInfo) SetInstance(ins discovery.Instance) {
	ri.Lock()
	ri.instance = ins
	ri.Unlock()
}

// ServiceName implements the rpcinfo.EndpointInfo interface.
func (ri *remoteInfo) ServiceName() string {
	return ri.serviceName
}

// SetServiceName implements the RemoteInfo interface.
func (ri *remoteInfo) SetServiceName(name string) {
	ri.serviceName = name
}

// Method implements the rpcinfo.EndpointInfo interface.
func (ri *remoteInfo) Method() string {
	return ri.method
}

// Tag implements the rpcinfo.EndpointInfo interface.
func (ri *remoteInfo) Tag(key string) (value string, exist bool) {
	ri.RLock()
	defer ri.RUnlock()

	if ri.instance != nil {
		if value, exist = ri.instance.Tag(key); exist {
			return
		}
	}
	value, exist = ri.tags[key]
	return
}

// DefaultTag implements the rpcinfo.EndpointInfo interface.
func (ri *remoteInfo) DefaultTag(key, def string) string {
	if value, exist := ri.Tag(key); exist && value != "" {
		return value
	}
	return def
}

// SetRemoteAddr implements the RemoteAddrSetter interface.
func (ri *remoteInfo) SetRemoteAddr(addr net.Addr) bool {
	if ins, ok := ri.instance.(RemoteAddrSetter); ok {
		ri.Lock()
		defer ri.Unlock()
		return ins.SetRemoteAddr(addr)
	}
	return false
}

// Address implements the rpcinfo.EndpointInfo interface.
func (ri *remoteInfo) Address() net.Addr {
	ri.RLock()
	defer ri.RUnlock()
	if ri.instance != nil {
		return ri.instance.Address()
	}
	return nil
}

// SetTagLocks locks tag.
func (ri *remoteInfo) SetTagLock(key string) {
	ri.Lock()
	ri.tagLocks[key] = struct{}{}
	ri.Unlock()
}

// SetTag implements rpcinfo.MutableEndpointInfo.
func (ri *remoteInfo) SetTag(key, value string) error {
	ri.Lock()
	defer ri.Unlock()
	if _, exist := ri.tagLocks[key]; exist {
		return kerrors.ErrNotSupported
	}
	ri.tags[key] = value
	return nil
}

// ForceSetTag is used to set Tag without tag lock.
func (ri *remoteInfo) ForceSetTag(key, value string) {
	ri.Lock()
	defer ri.Unlock()
	ri.tags[key] = value
}

// CopyFrom copies the input RemoteInfo.
// the `from` param may be modified, so must do deep copy to prevent race.
func (ri *remoteInfo) CopyFrom(from RemoteInfo) {
	if from == nil || ri == from {
		return
	}
	ri.Lock()
	f := from.(*remoteInfo)
	ri.serviceName = f.serviceName
	ri.instance = f.instance
	ri.method = f.method
	ri.tags = f.copyTags()
	ri.tagLocks = f.copyTagsLocks()
	ri.Unlock()
}

// ImmutableView implements rpcinfo.MutableEndpointInfo.
func (ri *remoteInfo) ImmutableView() rpcinfo.EndpointInfo {
	return ri
}

func (ri *remoteInfo) copyTags() map[string]string {
	ri.Lock()
	defer ri.Unlock()
	newTags := make(map[string]string, len(ri.tags))
	for k, v := range ri.tags {
		newTags[k] = v
	}
	return newTags
}

func (ri *remoteInfo) copyTagsLocks() map[string]struct{} {
	ri.Lock()
	defer ri.Unlock()
	newTagLocks := make(map[string]struct{}, len(ri.tagLocks))
	for k, v := range ri.tagLocks {
		newTagLocks[k] = v
	}
	return newTagLocks
}

func (ri *remoteInfo) zero() {
	ri.Lock()
	defer ri.Unlock()
	ri.serviceName = ""
	ri.method = ""
	ri.instance = nil
	for k := range ri.tags {
		delete(ri.tags, k)
	}
	for k := range ri.tagLocks {
		delete(ri.tagLocks, k)
	}
}

// Recycle is used to recycle the remoteInfo.
func (ri *remoteInfo) Recycle() {
	if r, ok := ri.instance.(internal.Reusable); ok {
		r.Recycle()
	}
	ri.zero()
	remoteInfoPool.Put(ri)
}

// NewRemoteInfo creates a remoteInfo wrapping the given endpointInfo.
// The return value of the created RemoteInfo's Method method will be the `method` argument
// instead of the Method field in `basicInfo`.
// If the given basicInfo is nil, this function will panic.
func NewRemoteInfo(basicInfo *rpcinfo.EndpointBasicInfo, method string) RemoteInfo {
	if basicInfo == nil {
		panic(errors.New("nil basicInfo"))
	}
	ri := remoteInfoPool.Get().(*remoteInfo)
	ri.serviceName = basicInfo.ServiceName
	ri.method = method
	for k, v := range basicInfo.Tags {
		ri.tags[k] = v
	}
	return ri
}

// AsRemoteInfo converts an rpcinfo.EndpointInfo into a RemoteInfo. Returns nil if impossible.
func AsRemoteInfo(r rpcinfo.EndpointInfo) RemoteInfo {
	if v, ok := r.(RemoteInfo); ok {
		return v
	}
	return nil
}
