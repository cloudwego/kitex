package ttstream

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/cloudwego/kitex/internal/test"
)

// A late header frame that arrives after close() must be ignored. close() parks
// a terminal signal (streamSigNone/streamSigInactive) in the buffered headerSig
// channel; without the fix, onReadHeaderFrame falls into the non-blocking
// send's default branch and surfaces errUnexpectedHeader, which the read loop
// treats as fatal and tears down the whole mux transport connection.
func Test_onReadHeaderFrameAfterCloseIsIgnored(t *testing.T) {
	cs := newTestClientStream(context.Background())
	cs.close(nil, false, "", nil)

	late := &Frame{
		streamFrame: streamFrame{
			sid:    cs.sid,
			method: cs.method,
			header: map[string]string{"late": "header"},
		},
		typ: headerFrameType,
	}
	err := cs.onReadHeaderFrame(late)
	test.Assert(t, err == nil, "late header after close must be ignored, got err=%v", err)
	// the dropped late header must not leak into s.header
	test.Assert(t, cs.header == nil, "dropped header must not be stored")
}

// A genuine duplicate header (two header frames, no close in between) must
// still be reported as a protocol error. This guards against over-loosening:
// only the close-parked terminal signal path may be tolerated.
func Test_onReadDuplicateHeaderStillErrors(t *testing.T) {
	cs := newTestClientStream(context.Background())
	first := &Frame{streamFrame: streamFrame{sid: cs.sid, header: map[string]string{"a": "1"}}, typ: headerFrameType}
	test.Assert(t, cs.onReadHeaderFrame(first) == nil, "first header must be accepted")

	second := &Frame{streamFrame: streamFrame{sid: cs.sid, header: map[string]string{"b": "2"}}, typ: headerFrameType}
	err := cs.onReadHeaderFrame(second)
	test.Assert(t, err != nil, "duplicate header without close must still error")
	test.Assert(t, errors.Is(err, errUnexpectedHeader), "expected errUnexpectedHeader, got %v", err)
}

// Race oracle: hammer close() concurrently with onReadHeaderFrame. With the
// fix, onReadHeaderFrame must NEVER return errUnexpectedHeader once the stream
// has been closed (it either accepts the header before close, or ignores it
// after); without the fix the race detector run reproduces the spurious fatal.
func Test_onReadHeaderFrameRaceWithClose(t *testing.T) {
	const iterations = 200
	for i := 0; i < iterations; i++ {
		cs := newTestClientStream(context.Background())
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			cs.close(nil, false, "", nil)
		}()
		go func() {
			defer wg.Done()
			fr := &Frame{streamFrame: streamFrame{sid: cs.sid, header: map[string]string{"k": "v"}}, typ: headerFrameType}
			err := cs.onReadHeaderFrame(fr)
			// If the header wins the race it is accepted; if close wins, the
			// in-flight header is ignored. Neither ordering is a protocol error.
			if err != nil {
				t.Errorf("header/close race returned a fatal error: %v", err)
			}
		}()
		wg.Wait()
	}
}
