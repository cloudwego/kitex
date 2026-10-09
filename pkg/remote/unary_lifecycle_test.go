package remote

import (
	"net"
	"testing"

	"github.com/cloudwego/kitex/pkg/discovery"
	"github.com/cloudwego/kitex/pkg/remote/transmeta"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/rpcinfo/remoteinfo"
	"github.com/cloudwego/kitex/pkg/utils"
)

type unaryAddressConn struct {
	net.Conn
	addr net.Addr
}

func (c unaryAddressConn) RemoteAddr() net.Addr { return c.addr }

type unaryAddressInstance struct {
	discovery.Instance
	addr    net.Addr
	updates int
}

func (i *unaryAddressInstance) Address() net.Addr { return i.addr }

func (i *unaryAddressInstance) RefreshInstanceWithAddr(addr net.Addr) discovery.Instance {
	i.addr = addr
	i.updates++
	return i
}

func TestRefreshUnaryRPCInfoAddress(t *testing.T) {
	for _, role := range []RPCRole{Client, Server} {
		name := "client"
		if role == Server {
			name = "server"
		}
		t.Run(name, func(t *testing.T) {
			initial := utils.NewNetAddr("tcp", "127.0.0.1:8000")
			peer := utils.NewNetAddr("tcp", "127.0.0.1:9000")
			conn := &unaryAddressConn{addr: peer}
			from := rpcinfo.NewEndpointInfo("before", "old-method", initial, nil)
			to := remoteinfo.NewRemoteInfo(&rpcinfo.EndpointBasicInfo{}, "")
			instance := &unaryAddressInstance{addr: initial}
			to.SetInstance(instance)
			ri := rpcinfo.NewRPCInfo(from, to, rpcinfo.NewInvocation("", ""), rpcinfo.NewRPCConfig(), nil)
			defer rpcinfo.PutRPCInfo(ri)
			msg := NewMessage(nil, ri, Call, role)
			defer RecycleMessage(msg)
			h := msg.TransInfo().TransStrInfo()
			h[transmeta.HeaderTransRemoteAddr] = initial.String()
			msg.TransInfo().TransIntInfo()[transmeta.FromService] = "after"
			msg.TransInfo().TransIntInfo()[transmeta.FromMethod] = "new-method"
			endpoint := rpcinfo.EndpointInfo(to)
			if role == Server {
				endpoint = from
			}
			if n := testing.AllocsPerRun(100, func() { RefreshUnaryRPCInfo(msg, conn) }); n != 0 {
				t.Errorf("unchanged address allocated %v objects per refresh", n)
			}
			if endpoint.Address() != initial || instance.updates != 0 {
				t.Fatal("unchanged address was replaced")
			}
			if role == Server && (from.ServiceName() != "after" || from.Method() != "new-method") {
				t.Fatal("address optimization skipped source metadata updates")
			}
			h[transmeta.HeaderTransRemoteAddr] = "127.0.0.1:8001"
			RefreshUnaryRPCInfo(msg, conn)
			if endpoint.Address().Network() != "tcp" || endpoint.Address().String() != h[transmeta.HeaderTransRemoteAddr] {
				t.Fatal("changed header address was not applied")
			}
			delete(h, transmeta.HeaderTransRemoteAddr)
			RefreshUnaryRPCInfo(msg, conn)
			if endpoint.Address() != peer || conn.RemoteAddr() != peer {
				t.Fatal("deleted address did not restore the real peer")
			}
		})
	}
}
