package client

import (
	"errors"
	"fmt"

	internalclient "github.com/cloudwego/kitex/internal/client"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
	"github.com/cloudwego/kitex/pkg/utils"
	"github.com/cloudwego/kitex/transport"
)

func WithNativeTransport(t remote.NativeClientTransport) Option {
	return Option{F: func(o *internalclient.Options, di *utils.Slice) {
		if o.NativeTransportSet {
			o.NativeUnsupported = append(o.NativeUnsupported, "multiple native transports")
		}
		o.NativeTransportSet = true
		o.NativeTransport = t
		if t != nil {
			o.RemoteOpt.UnaryLifecycle = t
			o.MetaHandlers = append(o.MetaHandlers, t.BuiltinMetaHandlers()...)
		}
	}}
}

func (kc *kClient) checkNativeOptions() error {
	if kc.opt.NativeTransport == nil {
		return errors.New("native transport requires a bridge")
	}
	if len(kc.opt.NativeUnsupported) > 0 {
		return fmt.Errorf("native transport does not support %v", kc.opt.NativeUnsupported)
	}
	if kc.svcInfo.PayloadCodec != serviceinfo.Thrift || kc.opt.Configs.TransportProtocol() != transport.TTHeaderFramed {
		return errors.New("native transport requires TTHeaderFramed Thrift unary")
	}
	if kc.opt.Proxy != nil || kc.opt.WarmUpOption != nil || kc.opt.PoolCfg != nil || kc.opt.Resolver != nil || kc.opt.Targets != "" || kc.opt.Balancer != nil || kc.opt.BalancerCacheOpt != nil || len(kc.opt.IMWBs) != 0 || kc.opt.XDSEnabled {
		return errors.New("native transport does not support Go connection routing or pooling")
	}
	if kc.opt.UnaryOptions.RetryContainer != nil || len(kc.opt.UnaryOptions.RetryMethodPolicies) != 0 || kc.opt.UnaryOptions.RetryWithResult != nil {
		return errors.New("native transport does not support retry or backup")
	}
	return nil
}
