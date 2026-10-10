package remote

import (
	"context"

	"github.com/cloudwego/kitex/pkg/serviceinfo"
)

func PreparePrimaryHeaders(ctx context.Context, msg Message, response bool) bool {
	c := UnaryCallFromContext(ctx)
	if c == nil || msg == nil || msg.RPCInfo().Config().PayloadCodec() != serviceinfo.Thrift {
		return false
	}
	h, ok := c.Lifecycle.(interface{ PrimaryUnaryHeaders() bool })
	if !ok || !h.PrimaryUnaryHeaders() {
		return false
	}
	if m := msg.RPCInfo().Invocation().MethodInfo(); m != nil && (m.OneWay() || m.StreamingMode() != serviceinfo.StreamingNone) {
		return false
	}
	return AccessHeaders(msg.TransInfo()).EnablePrimary(response)
}
