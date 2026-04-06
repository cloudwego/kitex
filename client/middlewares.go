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

package client

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/cloudwego/kitex/internal"
	"github.com/cloudwego/kitex/pkg/discovery"
	"github.com/cloudwego/kitex/pkg/endpoint"
	"github.com/cloudwego/kitex/pkg/event"
	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/loadbalance/lbcache"
	"github.com/cloudwego/kitex/pkg/proxy"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/codec/protobuf"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/rpcinfo/remoteinfo"
)

const maxRetry = 6

func newProxyMW(prx proxy.ForwardProxy) endpoint.Middleware {
	// If you want to customize the processing logic of proxy middleware,
	// you can implement this interface to replace the default implementation.
	if p, ok := prx.(proxy.WithMiddleware); ok {
		return p.ProxyMiddleware()
	}
	return func(next endpoint.Endpoint) endpoint.Endpoint {
		return func(ctx context.Context, request, response interface{}) error {
			err := prx.ResolveProxyInstance(ctx)
			if err != nil {
				return err
			}
			err = next(ctx, request, response)
			return err
		}
	}
}

// instSnapshot captures the essential instance data as value types during Push,
// avoiding holding references to discovery.Instance (which may carry large tags maps).
type instSnapshot struct {
	network string
	address string
	weight  int
}

type discoveryEventExtra struct {
	cacheable bool
	cacheKey  string
	added     []instSnapshot
	updated   []instSnapshot
	removed   []instSnapshot
}

// KitexDumpLazyExtra implements lazyExtra interface in pkg/event
func (e *discoveryEventExtra) KitexDumpLazyExtra() interface{} {
	return map[string]interface{}{
		"Cacheable": e.cacheable,
		"CacheKey":  e.cacheKey,
		"Added":     formatSnapshots(e.added),
		"Updated":   formatSnapshots(e.updated),
		"Removed":   formatSnapshots(e.removed),
	}
}

func discoveryEventHandler(name string, bus event.Bus, queue event.Queue) func(d *discovery.Change) {
	return func(d *discovery.Change) {
		now := time.Now()
		bus.Dispatch(&event.Event{
			Name:  name,
			Time:  now,
			Extra: d,
		})
		queue.Push(&event.Event{
			Name: name,
			Time: now,
			Extra: &discoveryEventExtra{
				cacheable: d.Result.Cacheable,
				cacheKey:  d.Result.CacheKey,
				added:     snapshotInstances(d.Added),
				updated:   snapshotInstances(d.Updated),
				removed:   snapshotInstances(d.Removed),
			},
		})
	}
}

// newResolveMWBuilder creates a middleware for service discovery.
// This middleware selects an appropriate instance based on the resolver and loadbalancer given.
// If retryable error is encountered, it will retry until timeout or an unretryable error is returned.
func newResolveMWBuilder(lbf *lbcache.BalancerFactory) endpoint.MiddlewareBuilder {
	return func(ctx context.Context) endpoint.Middleware {
		return func(next endpoint.Endpoint) endpoint.Endpoint {
			return func(ctx context.Context, request, response interface{}) error {
				rpcInfo := rpcinfo.GetRPCInfo(ctx)

				dest := rpcInfo.To()
				if dest == nil {
					return kerrors.ErrNoDestService
				}

				remote := remoteinfo.AsRemoteInfo(dest)
				if remote == nil {
					err := fmt.Errorf("unsupported target EndpointInfo type: %T", dest)
					return kerrors.ErrInternalException.WithCause(err)
				}
				if remote.GetInstance() != nil {
					return next(ctx, request, response)
				}
				lb, err := lbf.Get(ctx, dest)
				if err != nil {
					return kerrors.ErrServiceDiscovery.WithCause(err)
				}

				var lastErr error
				for i := 0; i < maxRetry; i++ {
					select {
					case <-ctx.Done():
						return kerrors.ErrRPCTimeout
					default:
					}

					// we always need to get a new picker every time, because when downstream update deployment,
					// we may get an old picker that include all outdated instances which will cause connect always failed.
					picker := lb.GetPicker()
					ins := picker.Next(ctx, request)
					if ins == nil {
						err = kerrors.ErrNoMoreInstance.WithCause(fmt.Errorf("last error: %w", lastErr))
					} else {
						remote.SetInstance(ins)
						// TODO: generalize retry strategy
						err = next(ctx, request, response)
					}
					if r, ok := picker.(internal.Reusable); ok {
						r.Recycle()
					}
					if err == nil {
						return nil
					}
					if retryable(err) {
						lastErr = err
						klog.CtxWarnf(ctx, "KITEX: auto retry retryable error, to_service=%s, retry=%d error=%s", dest.ServiceName(), i+1, err.Error())
						continue
					}
					return err
				}
				return lastErr
			}
		}
	}
}

// newIOErrorHandleMW provides a hook point for io error handling.
func newIOErrorHandleMW(errHandle func(context.Context, error) error) endpoint.Middleware {
	if errHandle == nil {
		errHandle = DefaultClientErrorHandler
	}
	return func(next endpoint.Endpoint) endpoint.Endpoint {
		return func(ctx context.Context, request, response interface{}) (err error) {
			err = next(ctx, request, response)
			if err == nil {
				return
			}
			return errHandle(ctx, err)
		}
	}
}

func isRemoteErr(err error) bool {
	if err == nil {
		return false
	}
	switch err.(type) {
	// for thrift、KitexProtobuf, actually check *remote.TransError is enough
	case *remote.TransError, protobuf.PBError:
		return true
	default:
		// case thrift.TApplicationException ?
		// XXX: we'd like to get rid of apache pkg, should be ok to check by type name
		// for thrift v0.13.0, it's "*thrift.tApplicationException"
	}
	return strings.HasSuffix(reflect.TypeOf(err).String(), "ApplicationException")
}

// DefaultClientErrorHandler is Default ErrorHandler for client
// when no ErrorHandler is specified with Option `client.WithErrorHandler`, this ErrorHandler will be injected.
// for thrift、KitexProtobuf, >= v0.4.0 wrap protocol error to TransError, which will be more friendly.
func DefaultClientErrorHandler(ctx context.Context, err error) error {
	if isRemoteErr(err) {
		// Add 'remote' prefix to distinguish with local err.
		// Because it cannot make sure which side err when decode err happen
		return kerrors.ErrRemoteOrNetwork.WithCauseAndExtraMsg(err, "remote")
	}
	return kerrors.ErrRemoteOrNetwork.WithCause(err)
}

// ClientErrorHandlerWithAddr is ErrorHandler for client, which will add remote addr info into error
func ClientErrorHandlerWithAddr(ctx context.Context, err error) error {
	addrStr := getRemoteAddr(ctx)
	if isRemoteErr(err) {
		// Add 'remote' prefix to distinguish with local err.
		// Because it cannot make sure which side err when decode err happen
		extraMsg := "remote"
		if addrStr != "" {
			extraMsg = "remote-" + addrStr
		}
		return kerrors.ErrRemoteOrNetwork.WithCauseAndExtraMsg(err, extraMsg)
	}
	return kerrors.ErrRemoteOrNetwork.WithCauseAndExtraMsg(err, addrStr)
}

type instInfo struct {
	Address string
	Weight  int
}

// snapshotInstances captures instance data as value types in a single slice allocation.
// This avoids holding discovery.Instance references (and their potentially large tags maps).
func snapshotInstances(insts []discovery.Instance) []instSnapshot {
	if len(insts) == 0 {
		return nil
	}
	snapshots := make([]instSnapshot, len(insts))
	for i, inst := range insts {
		addr := inst.Address()
		snapshots[i] = instSnapshot{
			network: addr.Network(),
			address: addr.String(),
			weight:  inst.Weight(),
		}
	}
	return snapshots
}

// formatSnapshots converts snapshots to []*instInfo during Dump (cold path).
func formatSnapshots(snapshots []instSnapshot) []*instInfo {
	if len(snapshots) == 0 {
		return nil
	}
	infos := make([]*instInfo, len(snapshots))
	for i := range snapshots {
		infos[i] = &instInfo{
			Address: snapshots[i].network + "://" + snapshots[i].address,
			Weight:  snapshots[i].weight,
		}
	}
	return infos
}

func retryable(err error) bool {
	return errors.Is(err, kerrors.ErrGetConnection) || errors.Is(err, kerrors.ErrCircuitBreak)
}

func getRemoteAddr(ctx context.Context) string {
	if ri := rpcinfo.GetRPCInfo(ctx); ri != nil && ri.To() != nil && ri.To().Address() != nil {
		return ri.To().Address().String()
	}
	return ""
}
