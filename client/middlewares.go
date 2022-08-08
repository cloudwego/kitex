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
	"time"

	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/xds/xdssuite"

	"github.com/apache/thrift/lib/go/thrift"
	"github.com/cloudwego/kitex/internal"
	"github.com/cloudwego/kitex/pkg/discovery"
	"github.com/cloudwego/kitex/pkg/endpoint"
	"github.com/cloudwego/kitex/pkg/event"
	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/cloudwego/kitex/pkg/loadbalance/lbcache"
	"github.com/cloudwego/kitex/pkg/proxy"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/codec/protobuf"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/rpcinfo/remoteinfo"
)

func newProxyMW(prx proxy.ForwardProxy) endpoint.Middleware {
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
			Extra: map[string]interface{}{
				"Added":   wrapInstances(d.Added),
				"Updated": wrapInstances(d.Updated),
				"Removed": wrapInstances(d.Removed),
			},
		})
	}
}

// newXDSRouterMWBuilder creates a middleware for request routing via XDS.
// This middleware picks one upstream cluster and sets RPCTimeout based on route config retrieved from XDS.
func newXDSRouterMWBuilder() endpoint.MiddlewareBuilder {
	router := xdssuite.NewXDSRouter()
	return func(ctx context.Context) endpoint.Middleware {
		return func(next endpoint.Endpoint) endpoint.Endpoint {
			return func(ctx context.Context, request, response interface{}) error {
				ri := rpcinfo.GetRPCInfo(ctx)
				dest := ri.To()
				if dest == nil {
					return kerrors.ErrNoDestService
				}
				res, err := router.Route(ctx, ri)
				if err != nil {
					klog.Errorf("[XDS] Router, route failed, error=%s", err)
					return err
				}
				// set destination
				_ = remoteinfo.AsRemoteInfo(dest).SetTag(xdssuite.RouterClusterKey, res.ClusterPicked)
				remoteinfo.AsRemoteInfo(dest).SetTagLock(xdssuite.RouterClusterKey)
				// set timeout
				_ = rpcinfo.AsMutableRPCConfig(ri.Config()).SetRPCTimeout(res.RPCTimeout)
				ctx = rpcinfo.NewCtxWithRPCInfo(ctx, ri)
				return next(ctx, request, response)
			}
		}
	}
}

// newResolveMWBuilder creates a middleware for service discovery.
// This middleware selects an appropriate instance based on the resolver and loadbalancer given.
// If retryable error is encountered, it will retry until timeout or an unretryable error is returned.
func newResolveMWBuilder(lbf *lbcache.BalancerFactory) endpoint.MiddlewareBuilder {
	return func(ctx context.Context) endpoint.Middleware {
		retryable := func(err error) bool {
			return errors.Is(err, kerrors.ErrGetConnection) || errors.Is(err, kerrors.ErrCircuitBreak)
		}

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

				picker := lb.GetPicker()
				if r, ok := picker.(internal.Reusable); ok {
					defer r.Recycle()
				}
				var lastErr error
				for {
					select {
					case <-ctx.Done():
						return kerrors.ErrRPCTimeout
					default:
					}

					ins := picker.Next(ctx, request)
					if ins == nil {
						return kerrors.ErrNoMoreInstance.WithCause(fmt.Errorf("last error: %w", lastErr))
					}
					remote.SetInstance(ins)

					// TODO: generalize retry strategy
					if err = next(ctx, request, response); err != nil && retryable(err) {
						lastErr = err
						continue
					}
					return err
				}
			}
		}
	}
}

// newIOErrorHandleMW provides a hook point for io error handling.
func newIOErrorHandleMW(errHandle func(error) error) endpoint.Middleware {
	if errHandle == nil {
		errHandle = DefaultClientErrorHandler
	}
	return func(next endpoint.Endpoint) endpoint.Endpoint {
		return func(ctx context.Context, request, response interface{}) (err error) {
			err = next(ctx, request, response)
			if err == nil {
				return
			}
			return errHandle(err)
		}
	}
}

// DefaultClientErrorHandler is Default ErrorHandler for client
// when no ErrorHandler is specified with Option `client.WithErrorHandler`, this ErrorHandler will be injected.
// for thrift、KitexProtobuf, >= v0.4.0 wrap protocol error to TransError, which will be more friendly.
func DefaultClientErrorHandler(err error) error {
	switch err.(type) {
	// for thrift、KitexProtobuf, actually check *remote.TransError is enough
	case *remote.TransError, thrift.TApplicationException, protobuf.PBError:
		// Add 'remote' prefix to distinguish with local err.
		// Because it cannot make sure which side err when decode err happen
		return kerrors.ErrRemoteOrNetwork.WithCauseAndExtraMsg(err, "remote")
	}
	return kerrors.ErrRemoteOrNetwork.WithCause(err)
}

type instInfo struct {
	Address string
	Weight  int
}

func wrapInstances(insts []discovery.Instance) []*instInfo {
	if len(insts) == 0 {
		return nil
	}
	instInfos := make([]*instInfo, 0, len(insts))
	for i := range insts {
		inst := insts[i]
		addr := fmt.Sprintf("%s://%s", inst.Address().Network(), inst.Address().String())
		instInfos = append(instInfos, &instInfo{Address: addr, Weight: inst.Weight()})
	}
	return instInfos
}
