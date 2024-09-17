package ttstream

import (
	"io"
	"sync"
)

type streamIO struct {
	stream *stream
	cond   *sync.Cond
	frames []*Frame
	end    bool
}

func newStreamIO(s *stream) *streamIO {
	var lock sync.Mutex
	var cond = sync.NewCond(&lock)
	return &streamIO{stream: s, cond: cond}
}

func (s *streamIO) input(f *Frame) {
	s.cond.L.Lock()
	s.frames = append(s.frames, f)
	s.cond.L.Unlock()
	s.cond.Signal()
}

func (s *streamIO) output() (f *Frame, err error) {
	s.cond.L.Lock()
	for len(s.frames) == 0 && !s.end {
		s.cond.Wait()
	}
	// have incoming frames or eof
	if len(s.frames) == 0 && s.end {
		return f, io.EOF
	}
	f = s.frames[0]
	s.frames = s.frames[1:]
	s.cond.L.Unlock()
	return f, nil
}

func (s *streamIO) eof() {
	s.cond.L.Lock()
	s.end = true
	s.cond.L.Unlock()
	s.cond.Signal()
}
