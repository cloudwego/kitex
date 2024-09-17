package ttstream

import (
	"context"
	"errors"
	"fmt"
	"io"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cloudwego/gopkg/bufiox"
	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
	"github.com/cloudwego/kitex/pkg/streamx/provider/ttstream/container"
	"github.com/cloudwego/netpoll"
)

const (
	clientTransport int32 = 1
	serverTransport int32 = 2

	streamCacheSize = 32
	frameCacheSize  = 32
)

var _ Object = (*transport)(nil)

type transport struct {
	kind    int32
	sinfo   *serviceinfo.ServiceInfo
	conn    netpoll.Connection
	reader  bufiox.Reader
	writer  bufiox.Writer
	streams sync.Map                 // key=streamID val=streamIO
	spipe   *container.Pipe[*stream] // in-coming stream channel
	scache  []*stream                // size is streamCacheSize
	wpipe   *container.Pipe[*Frame]  // out-coming frame channel
	stop    chan struct{}

	// for scavenger check
	lastActive atomic.Value // time.Time
}

func newTransport(kind int32, sinfo *serviceinfo.ServiceInfo, conn netpoll.Connection) *transport {
	_ = conn.SetDeadline(time.Now().Add(time.Hour))
	cbuffer := newConnBuffer(conn)
	t := &transport{
		kind:    kind,
		sinfo:   sinfo,
		conn:    conn,
		reader:  cbuffer,
		writer:  cbuffer,
		streams: sync.Map{},
		spipe:   container.NewPipe[*stream](),
		scache:  make([]*stream, 0, streamCacheSize),
		wpipe:   container.NewPipe[*Frame](),
		stop:    make(chan struct{}),
	}
	go func() {
		err := t.loopRead()
		if err != nil && errors.Is(err, io.EOF) {
			klog.Warnf("trans loop read err: %v", err)
		}
	}()
	go func() {
		err := t.loopWrite()
		if err != nil && errors.Is(err, io.EOF) {
			klog.Warnf("trans loop write err: %v", err)
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
		now := time.Now()
		t.lastActive.Store(now)

		// decode frame
		fr, err := DecodeFrame(context.Background(), t.reader)
		if err != nil {
			return err
		}

		switch fr.typ {
		case metaFrameType:
			sio, ok := t.loadStreamIO(fr.sid)
			if !ok {
				return fmt.Errorf("transport[%d] read a unknown stream meta: sid=%d", t.kind, fr.sid)
			}
			err = sio.stream.readMetaFrame(fr.meta, fr.header, fr.payload)
			if err != nil {
				return err
			}
		case headerFrameType:
			switch t.kind {
			case serverTransport:
				// Header Frame: server recv a new stream
				smode := t.sinfo.MethodInfo(fr.method).StreamingMode()
				s := newStream(t, smode, fr.streamFrame)
				klog.Debugf("transport[%d] read a new stream: sid=%d", t.kind, s.sid)
				t.spipe.Write(s)
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
			// Trailer Frame: recv trailer, Close read direction
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

func (t *transport) writeFrame(frame *Frame, batchFlush bool) error {
	err := EncodeFrame(context.Background(), t.writer, frame)
	if err != nil {
		return err
	}
	if batchFlush {
		return nil
	}
	return t.writer.Flush()
}

func (t *transport) loopWrite() error {
	frames := make([]*Frame, frameCacheSize)
	for {
		now := time.Now()
		t.lastActive.Store(now)

		n, err := t.wpipe.Read(frames)
		if err != nil {
			if errors.Is(err, container.PipeEOF) {
				return nil
			}
			return err
		}
		// if n > 1, we should use batch writes
		batchFlush := n > 1
		for i := 0; i < n; i++ {
			if err := t.writeFrame(frames[i], batchFlush); err != nil {
				return err
			}
		}
		if batchFlush {
			if err := t.writer.Flush(); err != nil {
				return err
			}
		} else {
			// wait for write happens
			runtime.Gosched()
		}
	}
}

func (t *transport) Available() bool {
	v := t.lastActive.Load()
	if v == nil {
		return true
	}
	lastActive := v.(time.Time)
	// let unavailable time configurable
	return time.Now().Sub(lastActive) < time.Minute*10
}

func (t *transport) Close() (err error) {
	select {
	case <-t.stop:
	default:
		klog.Warnf("transport[%s] is closing", t.conn.LocalAddr())
		close(t.stop)
		t.spipe.Close()
		t.wpipe.Close()
		t.conn.Close()
	}
	return nil
}

func (t *transport) streamSend(sid int32, method string, wheader Header, payload []byte) (err error) {
	if len(wheader) > 0 {
		err = t.streamSendHeader(sid, method, wheader)
		if err != nil {
			return err
		}
	}
	f := newFrame(streamFrame{sid: sid, method: method}, dataFrameType, payload)
	t.wpipe.Write(f)
	return nil
}

func (t *transport) streamSendHeader(sid int32, method string, header Header) (err error) {
	f := newFrame(streamFrame{sid: sid, method: method, header: header}, headerFrameType, []byte{})
	t.wpipe.Write(f)
	return nil
}

func (t *transport) streamSendTrailer(sid int32, method string, trailer Trailer) (err error) {
	f := newFrame(streamFrame{sid: sid, method: method, trailer: trailer}, trailerFrameType, []byte{})
	t.wpipe.Write(f)
	return nil
}

func (t *transport) streamRecv(sid int32) (payload []byte, err error) {
	sio, ok := t.loadStreamIO(sid)
	if !ok {
		return nil, io.EOF
	}
	f, err := sio.output()
	if err != nil {
		return nil, err
	}
	payload = f.payload
	recycleFrame(f)
	return payload, nil
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
// newStream is concurrency safe
func (t *transport) newStream(
	ctx context.Context, method string, header map[string]string) (*stream, error) {
	if t.kind != clientTransport {
		return nil, fmt.Errorf("transport already be used as other kind")
	}
	sid := atomic.AddInt32(&clientStreamID, 1)
	smode := t.sinfo.MethodInfo(method).StreamingMode()
	smeta := streamFrame{
		sid:    sid,
		method: method,
		header: header,
	}
	f := newFrame(smeta, headerFrameType, []byte{})
	s := newStream(t, smode, smeta)
	t.wpipe.Write(f) // create stream
	return s, nil
}

// readStream wait for a new incoming stream on current connection
// it's typically used by server side
func (t *transport) readStream() (*stream, error) {
	if t.kind != serverTransport {
		return nil, fmt.Errorf("transport already be used as other kind")
	}
READ:
	if len(t.scache) > 0 {
		s := t.scache[len(t.scache)-1]
		t.scache = t.scache[:len(t.scache)-1]
		return s, nil
	}
	n, err := t.spipe.Read(t.scache[0:streamCacheSize])
	if err != nil {
		if errors.Is(err, container.PipeEOF) {
			return nil, io.EOF
		}
		return nil, err
	}
	if n == 0 {
		panic("Assert: N == 0 !")
	}
	t.scache = t.scache[:n]
	goto READ
}
