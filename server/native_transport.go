package server

import (
	internalserver "github.com/cloudwego/kitex/internal/server"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/utils"
)

func WithNativeTransport(t remote.NativeServerTransport) Option {
	return Option{F: func(o *internalserver.Options, di *utils.Slice) {
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
