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

//
//func RecvHeader(cs streamx.ClientStream) (Header, error) {
//	csc, ok := cs.(streamx.ClientStreamContext[Header, Trailer])
//	if !ok {
//		return nil, ErrInvalidStreamKind
//	}
//	return csc.Header()
//}
//
//func RecvTrailer(cs streamx.ClientStream) (Trailer, error) {
//	csc, ok := cs.(streamx.ClientStreamContext[Header, Trailer])
//	if !ok {
//		return nil, ErrInvalidStreamKind
//	}
//	return csc.Trailer()
//}

// ServerStreamMeta cannot read header directly, should read from ctx
type ServerStreamMeta interface {
	streamx.ServerStream
	SetHeader(hd Header) error
	SendHeader(hd Header) error
	SetTrailer(hd Trailer) error
}

//
//func SetHeader(ss streamx.ServerStream, hd Header) error {
//	tss, ok := ss.(ServerStreamMeta)
//	if !ok {
//		return ErrInvalidStreamKind
//	}
//	return tss.SetHeader(hd)
//}
//
//func SendHeader(ss streamx.ServerStream, hd Header) error {
//	tss, ok := ss.(ServerStreamMeta)
//	if !ok {
//		return ErrInvalidStreamKind
//	}
//	return tss.SendHeader(hd)
//}
//
//func SetTrailer(ss streamx.ServerStream, hd Trailer) error {
//	tss, ok := ss.(ServerStreamMeta)
//	if !ok {
//		return ErrInvalidStreamKind
//	}
//	return tss.SetTrailer(hd)
//}
