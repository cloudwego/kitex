package remote

import (
	"context"
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cloudwego/kitex/pkg/remote/transmeta"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/rpcinfo/remoteinfo"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
	"github.com/cloudwego/kitex/pkg/utils"
	"github.com/cloudwego/kitex/transport"
)

var ErrUnaryLifecycleProtocol = errors.New("unary lifecycle requires TTHeader Thrift unary and the default codec")

type UnaryLifecycle interface {
	ClientPrepare(context.Context, *UnaryCall) error
	ClientFinish(context.Context, *UnaryCall) error
	ServerBegin(context.Context, *UnaryCall) error
	ServerResponse(context.Context, *UnaryCall) error
	ServerComplete(context.Context, *UnaryCall) error
}

type UnaryFailureStage uint8

const (
	UnaryFailureNone UnaryFailureStage = iota
	UnaryFailurePrepare
	UnaryFailureConnect
	UnaryFailureSend
	UnaryFailureReceive
	UnaryFailureBusiness
)

type UnaryCall struct {
	Request, Response               Message
	RequestWire, ResponseWire       UnaryWire
	State                           any
	Conn                            net.Conn
	ConnID                          uint64
	Err                             error
	FailureStage                    UnaryFailureStage
	Begin, ConnectStart, ConnectEnd time.Time
	WriteStart, WriteEnd, ReadEnd   time.Time
	HandlerStart, HandlerEnd        time.Time
	Reused                          bool
	MetaPrepared                    bool
	Started                         bool
	Finished                        bool
	Lifecycle                       UnaryLifecycle
	dialer                          Dialer
}

type UnaryWire struct {
	Framed      bool
	SeqID       int32
	ProtocolID  uint16
	Flags       uint16
	MessageType uint32
}

var unaryCallPool = sync.Pool{New: func() any { return new(UnaryCall) }}

var unaryConnID uint64

func NextUnaryConnID() uint64 {
	return atomic.AddUint64(&unaryConnID, 1)
}

type unaryCallKey struct{}

func NewUnaryCall(h UnaryLifecycle) *UnaryCall {
	c := unaryCallPool.Get().(*UnaryCall)
	c.Lifecycle = h
	c.Begin = time.Now()
	return c
}

func WithUnaryCall(ctx context.Context, c *UnaryCall) context.Context {
	return context.WithValue(ctx, unaryCallKey{}, c)
}

func UnaryCallFromContext(ctx context.Context) *UnaryCall {
	if ctx == nil {
		return nil
	}
	c, _ := ctx.Value(unaryCallKey{}).(*UnaryCall)
	return c
}

func (c *UnaryCall) Recycle() {
	*c = UnaryCall{}
	unaryCallPool.Put(c)
}

func (c *UnaryCall) FinishClient(ctx context.Context, err error) error {
	if !c.Started || c.Finished {
		return err
	}
	c.Finished, c.Err = true, err
	if e := c.Lifecycle.ClientFinish(ctx, c); err == nil {
		err = e
	}
	return err
}

func (c *UnaryCall) CompleteServer(ctx context.Context, err error) error {
	if !c.Started || c.Finished {
		return err
	}
	c.Finished = true
	if err != nil {
		c.Err = err
	}
	if e := c.Lifecycle.ServerComplete(ctx, c); err == nil {
		err = e
	}
	return err
}

func (c *UnaryCall) Dialer(d Dialer) Dialer {
	c.dialer = d
	c.Reused = true
	return c
}

func (c *UnaryCall) DialTimeout(network, address string, timeout time.Duration) (net.Conn, error) {
	c.Reused = false
	return c.dialer.DialTimeout(network, address, timeout)
}

func CheckUnaryLifecycle(ri rpcinfo.RPCInfo) error {
	tp := ri.Config().TransportProtocol()
	m := ri.Invocation().MethodInfo()
	if ri.Config().PayloadCodec() != serviceinfo.Thrift || (tp != transport.TTHeader && tp != transport.TTHeaderFramed) ||
		(m != nil && (m.OneWay() || m.StreamingMode() != serviceinfo.StreamingNone)) {
		return ErrUnaryLifecycleProtocol
	}
	return nil
}

func RefreshUnaryRPCInfo(msg Message, conn net.Conn) {
	ri := msg.RPCInfo()
	endpoint := ri.To()
	if msg.RPCRole() == Server {
		endpoint = ri.From()
	}
	addr := conn.RemoteAddr()
	changed := true
	if v := msg.TransInfo().TransStrInfo()[transmeta.HeaderTransRemoteAddr]; v != "" {
		if endpoint != nil {
			current := endpoint.Address()
			changed = current == nil || current.Network() != "tcp" || current.String() != v
		}
		if changed {
			addr = utils.NewNetAddr("tcp", v)
		}
	}
	if msg.RPCRole() == Server {
		if from := rpcinfo.AsMutableEndpointInfo(ri.From()); from != nil {
			if changed {
				from.SetAddress(addr)
			}
			from.SetServiceName(msg.TransInfo().TransIntInfo()[transmeta.FromService])
			from.SetMethod(msg.TransInfo().TransIntInfo()[transmeta.FromMethod])
		}
	} else if changed {
		if to := remoteinfo.AsRemoteInfo(ri.To()); to != nil {
			to.SetRemoteAddr(addr)
		}
	}
}
