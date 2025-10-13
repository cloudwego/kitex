package trans

import (
	"net"

	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/utils"
)

// NewMultipleTransServerFactory creates a TransServerFactory that creates multiple TransServer.
func NewMultipleTransServerFactory(factories []remote.TransServerFactory) remote.TransServerFactory {
	if len(factories) == 0 {
		panic("no factories")
	}
	if len(factories) == 1 {
		return factories[0]
	}
	return &multipleTransServerFactory{
		factories: factories,
	}
}

type multipleTransServerFactory struct {
	factories []remote.TransServerFactory
}

func (f *multipleTransServerFactory) NewTransServer(opt *remote.ServerOption, transHdlr remote.ServerTransHandler) remote.TransServer {
	svr := &multipleTransServer{
		svrs: make([]remote.TransServer, 0, len(f.factories)),
	}
	for _, factory := range f.factories {
		svr.svrs = append(svr.svrs, factory.NewTransServer(opt, transHdlr))
	}
	return svr
}

type multipleTransServer struct {
	svrs []remote.TransServer

	lns []net.Listener
}

var _ remote.TransServer = &multipleTransServer{}

func (ts *multipleTransServer) CreateListener(addr net.Addr) (net.Listener, error) {
	for _, svr := range ts.svrs {
		ln, err := svr.CreateListener(addr)
		if err != nil {
			return nil, err
		}
		ts.lns = append(ts.lns, ln)
	}
	return ts.lns[0], nil
}

func (ts *multipleTransServer) BootstrapServer(_ net.Listener) (err error) {
	errCh := make(chan error, len(ts.svrs))
	for i := range ts.svrs {
		svr := ts.svrs[i]
		ln := ts.lns[i]
		go func() { errCh <- svr.BootstrapServer(ln) }()
	}
	err = <-errCh
	if err != nil {
		klog.Errorf("KITEX: multiple listener server failed, err=%s", err.Error())
	}
	return err
}

func (ts *multipleTransServer) Shutdown() (err error) {
	for _, svr := range ts.svrs {
		svr.Shutdown()
	}
	return
}

func (ts *multipleTransServer) ConnCount() utils.AtomicInt {
	var cnt utils.AtomicInt
	for _, svr := range ts.svrs {
		cnt += svr.ConnCount()
	}
	return cnt
}
