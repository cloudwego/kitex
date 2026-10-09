package codec

import (
	"context"

	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/transport"
)

func (c *defaultCodec) PrepareUnaryWire(_ context.Context, msg remote.Message, call *remote.UnaryCall) error {
	if err := remote.CheckUnaryLifecycle(msg.RPCInfo()); err != nil {
		return err
	}
	// TODO: Provide payload bytes only when capture requests them.
	wire := remote.UnaryWire{
		Framed:      msg.RPCInfo().Config().TransportProtocol()&transport.Framed != 0,
		SeqID:       msg.RPCInfo().Invocation().SeqID(),
		ProtocolID:  uint16(getProtocolID(msg.RPCInfo().Config().PayloadCodec())),
		Flags:       uint16(getFlags(msg)),
		MessageType: uint32(msg.MessageType()),
	}
	if msg.RPCRole() == remote.Client {
		call.RequestWire = wire
	} else {
		call.ResponseWire = wire
	}
	return nil
}
