package ttstream

import (
	"context"
	"fmt"
	"github.com/cloudwego/gopkg/protocol/ttheader"
	"github.com/cloudwego/kitex/pkg/remote"
	nepolltrans "github.com/cloudwego/kitex/pkg/remote/trans/netpoll"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
	"github.com/cloudwego/netpoll"
	"io"
	"log"
	"sync"
	"sync/atomic"
)

type streamIO struct {
	stream *stream
	rch    chan Frame
}

const (
	clientTransport int32 = 1
	serverTransport int32 = 2
)

type transport struct {
	kind    int32
	sinfo   *serviceinfo.ServiceInfo
	conn    netpoll.Connection
	reader  remote.ByteBuffer
	writer  remote.ByteBuffer
	streams sync.Map     // key=streamID val=streamIO
	sch     chan *stream // in-coming stream channel
	wch     chan Frame   // out-coming frame channel
	stop    chan struct{}
}

func newTransport(kind int32, sinfo *serviceinfo.ServiceInfo, conn netpoll.Connection) *transport {
	reader := nepolltrans.NewReaderByteBuffer(conn.Reader())
	writer := nepolltrans.NewWriterByteBuffer(conn.Writer())
	t := &transport{
		kind:    kind,
		sinfo:   sinfo,
		conn:    conn,
		reader:  reader,
		writer:  writer,
		streams: sync.Map{},
		sch:     make(chan *stream),
		wch:     make(chan Frame),
		stop:    make(chan struct{}),
	}
	go func() {
		err := t.loopRead()
		if err != nil {
			log.Printf("loop read err: %v", err)
		}
	}()
	go func() {
		err := t.loopWrite()
		if err != nil {
			log.Printf("loop write err: %v", err)
		}
	}()
	return t
}

func (t *transport) storeStreamIO(s *stream) {
	t.streams.Store(s.sid, streamIO{stream: s, rch: make(chan Frame, 1024)})
}

func (t *transport) loadStreamIO(sid int32) (sio streamIO, ok bool) {
	val, ok := t.streams.Load(sid)
	if !ok {
		return sio, false
	}
	sio = val.(streamIO)
	return sio, true
}

func (t *transport) loopRead() error {
	for {
		// decode frame
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
				t.sch <- s
			case clientTransport:
				// Header Frame: client recv header
				sio, ok := t.loadStreamIO(fr.sid)
				if !ok {
					return fmt.Errorf("stream not found in stream map: sid=%d", fr.sid)
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
				return fmt.Errorf("stream not found in stream map: sid=%d", fr.sid)
			}
			sio.rch <- fr
		case trailerFrameType:
			// Trailer Frame: recv trailer, close read direction
			sio, ok := t.loadStreamIO(fr.sid)
			if !ok {
				return fmt.Errorf("stream not found in stream map: sid=%d", fr.sid)
			}
			if err = sio.stream.readTrailer(fr.trailer); err != nil {
				return err
			}
		}
	}
}

func (t *transport) loopWrite() error {
	for {
		select {
		case <-t.stop:
			// re check wch queue
			select {
			case frame := <-t.wch:
				err := EncodeFrame(context.Background(), t.writer, frame)
				if err != nil {
					return err
				}
			default:
				return t.conn.Close()
			}
		case frame := <-t.wch:
			err := EncodeFrame(context.Background(), t.writer, frame)
			if err != nil {
				return err
			}
		}
	}
}

func (t *transport) close() (err error) {
	select {
	case <-t.stop:
	default:
		close(t.stop)
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
	f := <-sio.rch
	log.Printf("trans stream recv: %v", f)
	if f.sid != s.sid { // f.sid == 0 means it's a empty frame
		return nil, io.EOF
	}
	return f.payload, nil
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
		return nil, io.EOF
	case s := <-t.sch:
		return s, nil
	}
}

func (t *transport) streamCloseRecv(s *stream) (err error) {
	sio, ok := t.loadStreamIO(s.sid)
	if !ok {
		return fmt.Errorf("stream not found in stream map: sid=%d", s.sid)
	}
	close(sio.rch)
	return nil
}
