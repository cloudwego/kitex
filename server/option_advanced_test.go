package server

import (
	"testing"
	"time"

	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
)

func TestWithServerBasicInfo(t *testing.T) {
	svcName := "svcName" + time.Now().String()
	svr := NewServer(WithServerBasicInfo(&rpcinfo.EndpointBasicInfo{
		ServiceName: svcName,
	}))
	iSvr := svr.(*server)
	test.Assert(t, iSvr.opt.Svr.ServiceName == svcName, iSvr.opt.Svr.ServiceName)
}
