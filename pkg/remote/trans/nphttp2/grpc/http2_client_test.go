package grpc

import (
	"context"
	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/codes"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/metadata"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/status"
	"golang.org/x/net/http2"
	"math"
	"testing"
)

func TestHttp2ClientTrailerRace(t *testing.T) {
	_, ct := setUp(t, 0, math.MaxUint32, normal)
	s, err := ct.NewStream(context.Background(), &CallHdr{})
	test.Assert(t, err == nil, err)

	go func() {
		s.Trailer()
	}()
	go func() {
		mdata := metadata.Pairs("k", "v")
		ct.closeStream(s, nil, false, http2.ErrCodeNo, status.New(codes.OK, ""), mdata, true)
	}()
}
