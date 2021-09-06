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

	"github.com/jackedelic/kitex/internal/client"
	"github.com/jackedelic/kitex/internal/internal"
	"github.com/jackedelic/kitex/internal/pkg/discovery"
	"github.com/jackedelic/kitex/internal/pkg/endpoint"
	"github.com/jackedelic/kitex/internal/pkg/event"
	"github.com/jackedelic/kitex/internal/pkg/kerrors"
	"github.com/jackedelic/kitex/internal/pkg/loadbalance"
	"github.com/jackedelic/kitex/internal/pkg/loadbalance/lbcache"
	"github.com/jackedelic/kitex/internal/pkg/proxy"
	"github.com/jackedelic/kitex/internal/pkg/rpcinfo"
	"github.com/jackedelic/kitex/internal/pkg/rpcinfo/remoteinfo"
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
			Extra: &discovery.Change{
				// Result not set to avoid memory leaking in debug port
				Added:   d.Added,
				Updated: d.Updated,
				Removed: d.Removed,
			},
		})
	}
}

// newResolveMWBuilder creates a middleware for service discovery.
// This middleware selects an appropriate instance based on the resolver and loadbalancer given.
// If retryable error is encountered, it will retry until timeout or an unretryable error is returned.
func newResolveMWBuilder(opt *client.Options) endpoint.MiddlewareBuilder {
	return func(ctx context.Context) endpoint.Middleware {
		bus := ctx.Value(endpoint.CtxEventBusKey).(event.Bus)
		events := ctx.Value(endpoint.CtxEventQueueKey).(event.Queue)

		onChange := discoveryEventHandler(discovery.ChangeEventName, bus, events)
		onDelete := discoveryEventHandler(discovery.DeleteEventName, bus, events)
		resolver := opt.Resolver
		if resolver == nil {
			resolver = &discovery.SynthesizedResolver{
				ResolveFunc: func(ctx context.Context, target string) (discovery.Result, error) {
					return discovery.Result{}, kerrors.ErrNoResolver
				},
				NameFunc: func() string { return "no_resolver" },
			}
		}
		balancer := opt.Balancer
		if balancer == nil {
			balancer = loadbalance.NewWeightedBalancer()
		}

		var cacheOpts lbcache.Options
		if opt.BalancerCacheOpt != nil {
			cacheOpts = *opt.BalancerCacheOpt
		}
		balancerFactory := lbcache.NewBalancerFactory(resolver, balancer, cacheOpts)
		rbIdx := balancerFactory.RegisterRebalanceHook(onChange)
		opt.CloseCallbacks = append(opt.CloseCallbacks, func() error {
			balancerFactory.DeregisterRebalanceHook(rbIdx)
			return nil
		})
		dIdx := balancerFactory.RegisterDeleteHook(onDelete)
		opt.CloseCallbacks = append(opt.CloseCallbacks, func() error {
			balancerFactory.DeregisterDeleteHook(dIdx)
			return nil
		})
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
				lb, err := balancerFactory.Get(ctx, dest)
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
		return endpoint.DummyMiddleware
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
