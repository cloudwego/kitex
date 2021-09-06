package transmeta

import (
	"testing"

	"github.com/jackedelic/kitex/internal/mocks"
	"github.com/jackedelic/kitex/internal/pkg/remote"
	"github.com/jackedelic/kitex/internal/pkg/rpcinfo"
	"github.com/jackedelic/kitex/internal/pkg/serviceinfo"
	"github.com/jackedelic/kitex/internal/test"
	"github.com/jackedelic/kitex/internal/transport"
)

func TestIsTTHeader(t *testing.T) {
	t.Run("with ttheader", func(t *testing.T) {
		ri := rpcinfo.NewRPCInfo(nil, nil, rpcinfo.NewInvocation("", ""), nil, rpcinfo.NewRPCStats())
		msg := remote.NewMessage(nil, mocks.ServiceInfo(), ri, remote.Call, remote.Server)
		msg.SetProtocolInfo(remote.NewProtocolInfo(transport.TTHeader, serviceinfo.Thrift))
		test.Assert(t, isTTHeader(msg))
	})
	t.Run("with ttheader framed", func(t *testing.T) {
		ri := rpcinfo.NewRPCInfo(nil, nil, rpcinfo.NewInvocation("", ""), nil, rpcinfo.NewRPCStats())
		msg := remote.NewMessage(nil, mocks.ServiceInfo(), ri, remote.Call, remote.Server)
		msg.SetProtocolInfo(remote.NewProtocolInfo(transport.TTHeaderFramed, serviceinfo.Thrift))
		test.Assert(t, isTTHeader(msg))
	})
}
