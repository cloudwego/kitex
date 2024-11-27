package jsonrpc

import (
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"

	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
	"github.com/cloudwego/netpoll"
)

type transport struct {
	sinfo   *serviceinfo.ServiceInfo
	conn    net.Conn
	streams sync.Map
	sch     chan *stream
	rch     map[int]chan Frame
	wch     chan Frame
	stop    chan struct{}
}

func newTransport(sinfo *serviceinfo.ServiceInfo, conn net.Conn) *transport {
	t := &transport{
		sinfo:   sinfo,
		conn:    conn,
		streams: sync.Map{},
		sch:     make(chan *stream),
		rch:     map[int]chan Frame{},
		wch:     make(chan Frame),
		stop:    make(chan struct{}),
	}
	go func() {
		err := t.loopRead()
		if err != nil && !errors.Is(err, net.ErrClosed) && !errors.Is(err, io.EOF) && !errors.Is(err, netpoll.ErrConnClosed) {
			klog.Debugf("transport loop read err: %v", err)
		}
	}()
	go func() {
		err := t.loopWrite()
		if err != nil && !errors.Is(err, net.ErrClosed) && !errors.Is(err, io.EOF) {
			klog.Debugf("transport loop write err: %v", err)
		}
	}()
	return t
}

func (t *transport) close() (err error) {
	close(t.stop)
	return nil
}

func (t *transport) streamSend(s *stream, payload []byte) (err error) {
	f := newFrame(frameTypeData, s.id, s.service, s.method, payload)
	t.wch <- f
	return nil
}

func (t *transport) streamRecv(s *stream) (payload []byte, err error) {
	f := <-t.rch[s.id]
	if f.sid != s.id { // f.sid == 0 means it's a empty frame
		return nil, io.EOF
	}
	return f.payload, nil
}

func (t *transport) loopRead() error {
	for {
		// decode frame
		frame, err := DecodeFrame(t.conn)
		if err != nil {
			return err
		}

		// prepare stream
		switch frame.typ {
		case frameTypeMeta: // new stream
			smode := t.sinfo.MethodInfo(frame.method).StreamingMode()
			s := newStream(t, frame.sid, smode, frame.service, frame.method)
			t.streams.Store(s.id, s)
			t.rch[s.id] = make(chan Frame, 1024)
			t.sch <- s
		case frameTypeData, frameTypeEOF: // stream streamRecv/close
			iss, ok := t.streams.Load(frame.sid)
			if !ok {
				return fmt.Errorf("stream not found in stream map: sid=%d", frame.sid)
			}
			s := iss.(*stream)
			switch frame.typ {
			case frameTypeEOF:
				err = s.recvEOF()
				return err
			case frameTypeData:
				// process data frame
				t.rch[s.id] <- frame
			}
		}
	}
}

func (t *transport) loopWrite() error {
	for {
		select {
		case <-t.stop:
			return nil
		case frame := <-t.wch:
			err := EncodeFrame(t.conn, frame)
			if err != nil {
				return err
			}
		}
	}
}

var clientStreamID uint32

func (t *transport) newStream(method string) (*stream, error) {
	sid := int(atomic.AddUint32(&clientStreamID, 1))
	smode := t.sinfo.MethodInfo(method).StreamingMode()
	service := t.sinfo.ServiceName
	f := newFrame(frameTypeMeta, sid, service, method, []byte{})
	s := newStream(t, sid, smode, service, method)
	t.streams.Store(s.id, s)
	t.rch[s.id] = make(chan Frame, 1024)
	t.wch <- f // create stream
	return s, nil
}

func (t *transport) streamCloseRecv(s *stream) (err error) {
	//for len(t.rch[s.id]) > 0 {
	//	runtime.Gosched()
	//}
	close(t.rch[s.id])
	return nil
}

func (t *transport) streamCloseSend(s *stream) (err error) {
	f := newFrame(frameTypeEOF, s.id, s.service, s.method, []byte("EOF"))
	t.wch <- f
	return nil
}

func (t *transport) readStream() (*stream, error) {
	select {
	case <-t.stop:
		return nil, io.EOF
	case s := <-t.sch:
		return s, nil
	}
}
