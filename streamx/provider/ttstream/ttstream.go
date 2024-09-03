package ttstream

import "github.com/cloudwego/kitex/streamx"

func NewGenericClientStream[Req, Res any](cs streamx.ClientStream) *streamx.GenericClientStream[Header, Trailer, Req, Res] {
	return streamx.NewGenericClientStream[Header, Trailer, Req, Res](cs)
}
