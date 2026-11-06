package codec

import (
	"context"

	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/transport"
)

// PrepareMessage exposes the envelope before encoding, allowing a custom
// endpoint to finish metadata preparation before it acquires a connection.
func (c *defaultCodec) PrepareMessage(ctx context.Context, msg remote.Message) error {
	if h := remote.MessageEventHandlerFromContext(ctx); h != nil {
		return h.OnMessageEvent(ctx, msg, remote.MessageEncode, remote.WireInfo{
			WithTTHeader: msg.RPCInfo().Config().TransportProtocol()&transport.TTHeader != 0,
			Framed:       msg.RPCInfo().Config().TransportProtocol()&transport.Framed != 0,
			SeqID:        msg.RPCInfo().Invocation().SeqID(),
			ProtocolID:   uint16(getProtocolID(msg.RPCInfo().Config().PayloadCodec())),
			Flags:        uint16(getFlags(msg)),
			MessageType:  uint32(msg.MessageType()),
		})
	}
	return nil
}
