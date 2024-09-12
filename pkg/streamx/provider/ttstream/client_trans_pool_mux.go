package ttstream

import (
	"sync"
	"time"

	"github.com/cloudwego/kitex/pkg/serviceinfo"
	"github.com/cloudwego/netpoll"
	"golang.org/x/sync/singleflight"
)

var _ transPool = (*muxTransPool)(nil)

func newMuxTransPool(sinfo *serviceinfo.ServiceInfo) transPool {
	t := new(muxTransPool)
	t.sinfo = sinfo
	t.scavenger = newScavenger()
	return t
}

type muxTransPool struct {
	sinfo     *serviceinfo.ServiceInfo
	pool      sync.Map // addr:netpoll.Connection
	scavenger *scavenger
	sflight   singleflight.Group
}

func (m *muxTransPool) Get(network string, addr string) (trans *transport, err error) {
	v, ok := m.pool.Load(addr)
	if ok {
		return v.(*transport), nil
	}
	v, err, _ = m.sflight.Do(addr, func() (interface{}, error) {
		conn, err := netpoll.DialConnection(network, addr, time.Second)
		if err != nil {
			return nil, err
		}
		trans = newTransport(clientTransport, m.sinfo, conn)
		m.scavenger.Add(trans)
		return trans, nil
	})
	if err != nil {
		return nil, err
	}
	return v.(*transport), nil
}

func (m *muxTransPool) Put(trans *transport) {
	// do nothing
}
