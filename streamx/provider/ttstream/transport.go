package ttstream

import (
	"context"
	"fmt"
	"github.com/cloudwego/gopkg/bufiox"
	"github.com/cloudwego/gopkg/protocol/ttheader"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
	"github.com/cloudwego/netpoll"
	"io"
	"log"
	"sync"
	"sync/atomic"
)

const (
	clientTransport int32 = 1
	serverTransport int32 = 2
)

type transport struct {
	kind    int32
	sinfo   *serviceinfo.ServiceInfo
	conn    netpoll.Connection
	reader  bufiox.Reader
	writer  bufiox.Writer
	streams sync.Map     // key=streamID val=streamIO
	sch     chan *stream // in-coming stream channel
	wch     chan Frame   // out-coming frame channel
	stop    chan struct{}
}

func newTransport(kind int32, sinfo *serviceinfo.ServiceInfo, conn netpoll.Connection) *transport {
	reader := bufiox.NewDefaultReader(conn)
	writer := bufiox.NewDefaultWriter(conn)
	t := &transport{
		kind:    kind,
		sinfo:   sinfo,
		conn:    conn,
		reader:  reader,
		writer:  writer,
		streams: sync.Map{},
		sch:     make(chan *stream, 8),
		wch:     make(chan Frame, 8),
		stop:    make(chan struct{}),
	}
	go func() {
		err := t.loopRead()
		if err != nil {
			log.Printf("loop read err: %v", err)
			return
		}
	}()
	go func() {
		err := t.loopWrite()
		if err != nil {
			log.Printf("loop write err: %v", err)
			return
		}
	}()
	return t
}

func (t *transport) storeStreamIO(s *stream) {
	t.streams.Store(s.sid, newStreamIO(s))
}

func (t *transport) loadStreamIO(sid int32) (sio *streamIO, ok bool) {
	val, ok := t.streams.Load(sid)
	if !ok {
		return sio, false
	}
	sio = val.(*streamIO)
	return sio, true
}

func (t *transport) loopRead() error {
	for {
		// decode frame
		log.Printf("loop read frame start")
		fr, err := DecodeFrame(context.Background(), t.reader)
		log.Printf("loop read frame=%v err=%v", fr, err)
		if err != nil {
			return err
		}

		switch fr.typ {
		case metaFrameType:
		case headerFrameType:
			switch t.kind {
			case serverTransport:
				// Header Frame: server recv a new stream
				smode := t.sinfo.MethodInfo(fr.method).StreamingMode()
				s := newStream(t, smode, fr.streamMeta)
				log.Printf("transport[%d] read a new stream: sid=%d", t.kind, s.sid)
				t.sch <- s
			case clientTransport:
				// Header Frame: client recv header
				sio, ok := t.loadStreamIO(fr.sid)
				if !ok {
					return fmt.Errorf("transport[%d] read a unknown stream header: sid=%d", t.kind, fr.sid)
				}
				err = sio.stream.readHeader(fr.header)
				if err != nil {
					return err
				}
			}
		case dataFrameType:
			// Data Frame: decode and distribute data
			sio, ok := t.loadStreamIO(fr.sid)
			if !ok {
				return fmt.Errorf("transport[%d] read a unknown stream data: sid=%d", t.kind, fr.sid)
			}
			sio.input(fr)
		case trailerFrameType:
			// Trailer Frame: recv trailer, close read direction
			sio, ok := t.loadStreamIO(fr.sid)
			if !ok {
				return fmt.Errorf("transport[%d] read a unknown stream trailer: sid=%d", t.kind, fr.sid)
			}
			if err = sio.stream.readTrailer(fr.trailer); err != nil {
				return err
			}
		}
	}
}

func (t *transport) writeFrame(frame Frame) error {
	log.Printf("loop stop and write frame start: %v", frame)
	err := EncodeFrame(context.Background(), t.writer, frame)
	log.Printf("loop stop and write frame end: %v", err)
	return err
}

func (t *transport) loopWrite() error {
	defer func() {
		log.Printf("loop write end: %s", t.conn.LocalAddr())
	}()
	for {
		select {
		case <-t.stop:
			// re-check wch queue
			select {
			case frame := <-t.wch:
				if err := t.writeFrame(frame); err != nil {
					return err
				}
			default:
				return nil
			}
		case frame := <-t.wch:
			if err := t.writeFrame(frame); err != nil {
				return err
			}
		}
	}
}

func (t *transport) close() (err error) {
	select {
	case <-t.stop:
	default:
		log.Printf("transport[%s] closeing", t.conn.LocalAddr())
		close(t.stop)
		t.conn.Close()
	}
	return nil
}

func (t *transport) streamSend(s streamMeta, payload []byte) (err error) {
	f := newFrame(s, dataFrameType, payload)
	t.wch <- f
	return nil
}

func (t *transport) streamSendHeader(s streamMeta, header Header) (err error) {
	s.header = header
	s.trailer = nil
	f := newFrame(s, headerFrameType, []byte{})
	t.wch <- f
	return nil
}

func (t *transport) streamSendTrailer(s streamMeta, trailer Trailer) (err error) {
	s.header = nil
	s.trailer = trailer
	f := newFrame(s, trailerFrameType, []byte{})
	t.wch <- f
	return nil
}

func (t *transport) streamRecv(s streamMeta) (payload []byte, err error) {
	sio, ok := t.loadStreamIO(s.sid)
	if !ok {
		return nil, io.EOF
	}
	f, err := sio.output()
	if err != nil {
		return nil, err
	}
	log.Printf("trans stream recv: %v", f)
	return f.payload, nil
}

func (t *transport) streamCloseRecv(s *stream) (err error) {
	sio, ok := t.loadStreamIO(s.sid)
	if !ok {
		return fmt.Errorf("stream not found in stream map: sid=%d", s.sid)
	}
	sio.eof()
	return nil
}

func (t *transport) streamClose(s *stream) (err error) {
	// remove stream from transport
	t.streams.Delete(s.sid)
	return nil
}

var clientStreamID int32

// newStream create new stream on current connection
// it's typically used by client side
func (t *transport) newStream(ctx context.Context, method string) (*stream, error) {
	if t.kind != clientTransport {
		return nil, fmt.Errorf("transport already be used as other kind")
	}
	sid := atomic.AddInt32(&clientStreamID, 1)
	smode := t.sinfo.MethodInfo(method).StreamingMode()
	smeta := streamMeta{
		sid:    sid,
		method: method,
		header: map[string]string{
			ttheader.HeaderIDLServiceName: t.sinfo.ServiceName,
		},
	}
	f := newFrame(smeta, headerFrameType, []byte{})
	s := newStream(t, smode, smeta)
	t.wch <- f // create stream
	return s, nil
}

// readStream wait for a new incoming stream on current connection
// it's typically used by server side
func (t *transport) readStream() (*stream, error) {
	if t.kind != serverTransport {
		return nil, fmt.Errorf("transport already be used as other kind")
	}
	select {
	case <-t.stop:
		println("transport return")
		return nil, io.EOF
	case s := <-t.sch:
		if s == nil {
			return nil, io.EOF
		}
		return s, nil
	}
}
