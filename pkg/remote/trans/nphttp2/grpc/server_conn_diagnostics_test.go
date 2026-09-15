package grpc

import (
	"context"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/net/http2"

	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/metadata"
	"github.com/cloudwego/kitex/pkg/remote/transmeta"
	"github.com/cloudwego/netpoll"
)

func diagnosticTestServer(t *testing.T) *http2Server {
	t.Helper()
	conn, peer := net.Pipe()
	t.Cleanup(func() { conn.Close(); peer.Close() })
	s := &http2Server{
		ctx: context.Background(), conn: conn, done: make(chan struct{}),
		localAddr: conn.LocalAddr(), remoteAddr: conn.RemoteAddr(),
		state: reachable, activeStreams: make(map[uint32]*Stream),
		diagnostics: newServerConnDiagnostics(time.Now().Add(-time.Second)),
	}
	s.controlBuf = newControlBuffer(s.done)
	return s
}

func diagnosticTestStream(t *http2Server, id uint32, caller, logID string) *Stream {
	ctx, cancel := context.WithCancel(context.Background())
	ctx, cancelReason := newContextWithCancelReason(ctx, cancel)
	s := &Stream{id: id, ctx: ctx, cancel: cancelReason, method: "/test.Service/Stream", sourceService: caller}
	t.initStreamDiagnostics(s)
	RecordServerStreamLogID(s.ctx, logID, "received", true)
	t.mu.Lock()
	t.activeStreams[id] = s
	t.mu.Unlock()
	return s
}

func TestConnectionDiagnosticsCloseSnapshot(t *testing.T) {
	for _, raw := range []error{io.EOF, io.ErrUnexpectedEOF, netpoll.ErrEOF} {
		t.Run(raw.Error(), func(t *testing.T) {
			s := diagnosticTestServer(t)
			a := diagnosticTestStream(s, 1, "caller.a", "log-a")
			b := diagnosticTestStream(s, 3, "caller.b", "log-b")
			atomic.StoreInt64(&s.diagnostics.lastFrameAt, time.Now().Add(-500*time.Millisecond).UnixNano())
			if err := s.closeWithDiagnostic(errConnectionEOF, raw, "read_frame"); err != nil {
				t.Fatal(err)
			}
			v := s.diagnostics.close
			if v == nil || v.RawError != raw.Error() || v.RawErrorType != fmt.Sprintf("%T", raw) || v.Stage != "read_frame" {
				t.Fatalf("original read error lost: %+v", v)
			}
			if v.StateBeforeClose != "reachable" || v.ActiveStreamCount != 2 || len(v.ActiveStreams) != 2 || v.LastFrameAgoMS < 450 {
				t.Fatalf("close snapshot taken too late: %+v", v)
			}
			for i, want := range []string{"log-a", "log-b"} {
				got := v.ActiveStreams[i]
				if got.LogID != want || got.SourceService != []string{"caller.a", "caller.b"}[i] || !got.MetadataRead || !got.IncomingLogIDPresent || got.Canceled {
					t.Fatalf("incorrect stream association: %+v", got)
				}
			}
			if a.ctx.Err() != errConnectionEOF || b.ctx.Err() != errConnectionEOF || s.activeStreams != nil {
				t.Fatal("original cancellation behavior changed")
			}
			RecordServerStreamLogID(a.ctx, "later-id", "context", false)
			_ = s.closeWithDiagnostic(errIdleClosing, nil, "keepalive_timeout")
			if s.diagnostics.close != v || v.ActiveStreams[0].LogID != "log-a" {
				t.Fatal("first close snapshot was mutated")
			}
		})
	}
}

func TestConnectionDiagnosticsIdleHistoryAndBounds(t *testing.T) {
	s := diagnosticTestServer(t)
	for id := uint32(1); id <= 12; id++ {
		stream := diagnosticTestStream(s, id, "caller", fmt.Sprint(id))
		stream.cancel(nil)
		stream.swapState(streamDone)
		s.deleteStream(stream, true)
	}
	s.idle = time.Now().Add(-time.Second)
	_ = s.closeWithDiagnostic(errConnectionEOF, io.EOF, "read_frame")
	v := s.diagnostics.close
	if len(v.ActiveStreams) != 0 || len(v.RecentFinished) != diagnosticRecentLimit || v.IdleMS < 900 {
		t.Fatalf("idle connection lost its bounded history: %+v", v)
	}
	if v.RecentFinished[0].StreamID != 5 || v.RecentFinished[7].StreamID != 12 || v.RecentFinished[0].EndedAtMS == 0 {
		t.Fatalf("incorrect recent history: %+v", v.RecentFinished)
	}

	s = diagnosticTestServer(t)
	for id := uint32(1); id <= diagnosticActiveLimit+5; id++ {
		diagnosticTestStream(s, id, strings.Repeat("x", 1024), fmt.Sprint(id))
	}
	_ = s.closeWithDiagnostic(errConnectionEOF, io.EOF, "read_frame")
	v = s.diagnostics.close
	if len(v.ActiveStreams) != diagnosticActiveLimit || v.ActiveStreamsOmitted != 5 || len(v.ActiveStreams[0].SourceService) != diagnosticStringLimit {
		t.Fatalf("unbounded close diagnostic: %+v", v)
	}
}

func TestConnectionDiagnosticsRSTDoesNotCloseConnection(t *testing.T) {
	s := diagnosticTestServer(t)
	a := diagnosticTestStream(s, 1, "caller.a", "log-a")
	b := diagnosticTestStream(s, 3, "caller.b", "log-b")
	s.handleRSTStream(&http2.RSTStreamFrame{FrameHeader: http2.FrameHeader{StreamID: 1}, ErrCode: http2.ErrCodeCancel})
	if s.diagnostics.close != nil || s.state != reachable || a.ctx.Err() == nil || b.ctx.Err() != nil {
		t.Fatal("RST of one stream was confused with connection close")
	}
	_ = s.closeWithDiagnostic(errConnectionEOF, io.EOF, "read_frame")
	v := s.diagnostics.close
	if v.ActiveStreamCount != 1 || v.ActiveStreams[0].LogID != "log-b" || len(v.RecentFinished) != 1 || v.RecentFinished[0].LogID != "log-a" {
		t.Fatalf("RST/EOF associations mixed: %+v", v)
	}
	if len(v.RecentEvents) != 1 || v.RecentEvents[0].Code != "CANCEL" || v.RecentEvents[0].StreamID != 1 {
		t.Fatalf("RST event missing: %+v", v.RecentEvents)
	}
}

func TestConnectionDiagnosticsConcurrentMetadataAndClose(t *testing.T) {
	s := diagnosticTestServer(t)
	var wg sync.WaitGroup
	for id := uint32(1); id <= 16; id++ {
		stream := diagnosticTestStream(s, id, "caller", "initial")
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				RecordServerStreamLogID(stream.ctx, fmt.Sprint(i), "generated", false)
				_ = stream.diagnosticSnapshot(time.Now())
			}
			stream.cancel(nil)
			s.deleteStream(stream, true)
		}()
	}
	_ = s.closeWithDiagnostic(errConnectionEOF, io.EOF, "read_frame")
	wg.Wait()
	if s.diagnostics.close == nil {
		t.Fatal("close was not diagnosed")
	}
}

func TestConnectionDiagnosticsDisabled(t *testing.T) {
	s := diagnosticTestServer(t)
	s.diagnostics = nil
	stream := diagnosticTestStream(s, 1, "caller", "log")
	s.recordDiagnosticEvent(connectionDiagnosticEvent{Kind: "rst_stream"})
	_ = s.closeWithDiagnostic(errConnectionEOF, io.EOF, "read_frame")
	if stream.diagnostics != nil || stream.ctx.Err() != errConnectionEOF {
		t.Fatal("disabled diagnostics changed transport behavior")
	}
}

// Exercise actual HTTP/2 header parsing, stream context propagation and the
// reader -> control buffer -> loopy writer EOF path on one multiplexed connection.
func TestConnectionDiagnosticsWireEOF(t *testing.T) {
	srv, client := setUpWithOptions(t, 0, &ServerConfig{ConnectionDiagnostics: true}, connectionDiagnostics, ConnectOptions{})
	defer srv.stop()
	defer client.Close(errSelfCloseForTest)
	for _, caller := range []string{"caller.a", "caller.b"} {
		ctx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs(transmeta.HTTPSourceService, caller, "x-test-log-id", "log-"+caller))
		if _, err := client.NewStream(ctx, &CallHdr{Host: "localhost", Method: "/test.Service/Stream"}); err != nil {
			t.Fatal(err)
		}
	}
	var transport *http2Server
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		srv.mu.Lock()
		for tr := range srv.conns {
			transport = tr.(*http2Server)
		}
		srv.mu.Unlock()
		if transport != nil {
			transport.mu.Lock()
			ready := len(transport.activeStreams) == 2
			if ready {
				for _, stream := range transport.activeStreams {
					ready = ready && stream.diagnosticSnapshot(time.Now()).MetadataRead
				}
			}
			transport.mu.Unlock()
			if ready {
				break
			}
		}
		time.Sleep(time.Millisecond)
	}
	if transport == nil {
		t.Fatal("server transport was not established")
	}
	client.Close(errSelfCloseForTest)
	select {
	case <-transport.readerDone:
	case <-time.After(3 * time.Second):
		t.Fatal("server did not observe peer EOF")
	}
	transport.mu.Lock()
	v := transport.diagnostics.close
	transport.mu.Unlock()
	if v == nil || v.Stage != "read_frame" || v.ActiveStreamCount != 2 || v.RawError == "" {
		t.Fatalf("EOF did not retain both requests: %+v", v)
	}
	for _, stream := range v.ActiveStreams {
		if stream.LogID != "log-"+stream.SourceService || stream.Method != "/test.Service/Stream" {
			t.Fatalf("wire metadata not associated: %+v", stream)
		}
	}
}
