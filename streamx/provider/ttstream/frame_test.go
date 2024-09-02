package ttstream

import (
	"context"
	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/remote"
	"testing"
)

func TestFrameCodec(t *testing.T) {
	rw := remote.NewReaderWriterBuffer(1024)

	wframe := newFrame(streamMeta{
		sid:    1,
		method: "method",
		header: map[string]string{"key": "value"},
	}, headerFrameType, []byte("hello world"))
	err := EncodeFrame(context.Background(), rw, wframe)
	test.Assert(t, err == nil, err)

	rframe, err := DecodeFrame(context.Background(), rw)
	test.Assert(t, err == nil, err)
	test.DeepEqual(t, string(wframe.payload), string(rframe.payload))
	test.DeepEqual(t, wframe, rframe)
}

func TestPayloadCodec(t *testing.T) {
	rmsg := new(TestRequest)
	rmsg.A = 1
	rmsg.B = "hello world"
	payload, err := EncodePayload(context.Background(), rmsg)
	test.Assert(t, err == nil, err)

	wmsg := new(TestRequest)
	err = DecodePayload(context.Background(), payload, wmsg)
	test.Assert(t, err == nil, err)
	test.DeepEqual(t, wmsg, rmsg)
}
