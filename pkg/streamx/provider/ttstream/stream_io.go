package ttstream

import (
	"context"
	"errors"
	"io"

	"github.com/cloudwego/kitex/pkg/streamx/provider/ttstream/container"
)

type streamIO struct {
	ctx     context.Context
	trigger chan struct{}
	stream  *stream
	fpipe   *container.Pipe[*Frame]
	fcache  [1]*Frame
	end     bool
}

func newStreamIO(ctx context.Context, s *stream) *streamIO {
	sio := new(streamIO)
	sio.ctx = ctx
	sio.trigger = make(chan struct{})
	sio.stream = s
	sio.fpipe = container.NewPipe[*Frame]()
	return sio
}

func (s *streamIO) input(f *Frame) {
	_ = s.fpipe.Write(f)
}

func (s *streamIO) output() (f *Frame, err error) {
	n, err := s.fpipe.Read(s.fcache[:])
	if err != nil {
		if errors.Is(err, container.ErrPipeEOF) {
			return nil, io.EOF
		}
		return nil, err
	}
	if n == 0 {
		return nil, io.EOF
	}
	return s.fcache[0], nil
}

func (s *streamIO) eof() {
	s.fpipe.Close()
}

func (s *streamIO) cancel() {
	s.fpipe.Cancel()
}
