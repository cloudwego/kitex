package generic

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"

	"github.com/cloudwego/dynamicgo/conv"
	"github.com/cloudwego/dynamicgo/conv/j2t"
	"github.com/cloudwego/dynamicgo/conv/t2j"
	"github.com/cloudwego/kitex/pkg/generic/descriptor"
	"github.com/cloudwego/kitex/pkg/generic/thrift"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/codec"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
	jsoniter "github.com/json-iterator/go"
)

var (
	_ remote.PayloadCodec = &httpThriftCodec{}
	_ Closer              = &httpThriftCodec{}
)

// HTTPRequest alias of descriptor HTTPRequest
type HTTPRequest = descriptor.HTTPRequest

// HTTPResponse alias of descriptor HTTPResponse
type HTTPResponse = descriptor.HTTPResponse

type httpThriftCodec struct {
	svcDsc              atomic.Value // *idl
	provider            DescriptorProvider
	codec               remote.PayloadCodec
	binaryWithBase64    bool
	enableDynamicgoReq  bool
	enableDynamicgoResp bool
	convOpts            conv.Options
	j2t                 j2t.BinaryConv
	t2j                 t2j.BinaryConv
}

func (c *httpThriftCodec) update() {
	for {
		svc, ok := <-c.provider.Provide()
		if !ok {
			return
		}
		c.svcDsc.Store(svc)
	}
}

func (c *httpThriftCodec) getMethod(req interface{}) (*Method, error) {
	svcDsc, ok := c.svcDsc.Load().(*descriptor.ServiceDescriptor)
	if !ok {
		return nil, errors.New("get method name failed, no ServiceDescriptor")
	}
	r, ok := req.(*HTTPRequest)
	if !ok {
		return nil, errors.New("req is invalid, need descriptor.HTTPRequest")
	}
	function, err := svcDsc.Router.Lookup(r)
	if err != nil {
		return nil, err
	}
	return &Method{function.Name, function.Oneway}, nil
}

func (c *httpThriftCodec) originalMarshal(ctx context.Context, msg remote.Message, out remote.ByteBuffer) error {
	svcDsc, ok := c.svcDsc.Load().(*descriptor.ServiceDescriptor)
	if !ok {
		return fmt.Errorf("get parser ServiceDescriptor failed")
	}

	inner := thrift.NewWriteHTTPRequest(svcDsc)
	inner.SetBinaryWithBase64(c.binaryWithBase64)
	msg.Data().(WithCodec).SetCodec(inner)
	return c.codec.Marshal(ctx, msg, out)
}

func (c *httpThriftCodec) originalUnmarshal(ctx context.Context, msg remote.Message, in remote.ByteBuffer) error {
	if err := codec.NewDataIfNeeded(serviceinfo.GenericMethod, msg); err != nil {
		return err
	}
	svcDsc, ok := c.svcDsc.Load().(*descriptor.ServiceDescriptor)
	if !ok {
		return fmt.Errorf("get parser ServiceDescriptor failed")
	}
	inner := thrift.NewReadHTTPResponse(svcDsc)
	inner.SetBase64Binary(c.binaryWithBase64)
	msg.Data().(WithCodec).SetCodec(inner)
	return c.codec.Unmarshal(ctx, msg, in)
}

func (c *httpThriftCodec) Close() error {
	return c.provider.Close()
}

var json = jsoniter.Config{
	EscapeHTML: true,
	UseNumber:  true,
}.Froze()
