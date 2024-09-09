package ttstream

import (
	"errors"
	"github.com/cloudwego/kitex/streamx"
)

var ErrInvalidStreamKind = errors.New("invalid stream kind")

type Header map[string]string
type Trailer map[string]string

// ClientStreamMeta cannot send header directly, should send from ctx
type ClientStreamMeta interface {
	streamx.ClientStream
	Header() (Header, error)
	Trailer() (Trailer, error)
}

// ServerStreamMeta cannot read header directly, should read from ctx
type ServerStreamMeta interface {
	streamx.ServerStream
	SetHeader(hd Header) error
	SendHeader(hd Header) error
	SetTrailer(hd Trailer) error
}
