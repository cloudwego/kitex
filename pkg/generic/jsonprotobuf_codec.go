package generic

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sync/atomic"

	gproto "github.com/cloudwego/kitex/pkg/generic/proto"
	"github.com/cloudwego/kitex/pkg/remote"

	"github.com/cloudwego/kitex/pkg/remote/codec/perrors"
	protob "github.com/golang/protobuf/proto"
)

type JSONData = []byte

type JSONToProtobufCodec struct {
	descriptorProvider PbDescriptorProvider
	codec              remote.PayloadCodec
	svcDsc             atomic.Value
}

var _ remote.PayloadCodec = &JSONToProtobufCodec{}

func NewJSONToProtobufCodec(descriptorProvider PbDescriptorProvider, codec remote.PayloadCodec) (*JSONToProtobufCodec, error) {
	c := &JSONToProtobufCodec{
		descriptorProvider: descriptorProvider,
		codec:              codec,
	}
	c.update()
	go c.watchServiceDescriptor()
	return c, nil
}

func (c *JSONToProtobufCodec) watchServiceDescriptor() {
	for svc := range c.descriptorProvider.Provide() {
		c.svcDsc.Store(svc)
	}
}

func (c *JSONToProtobufCodec) update() {
	svc, ok := <-c.descriptorProvider.Provide()
	if !ok {
		return
	}
	c.svcDsc.Store(svc)
}

func (c *JSONToProtobufCodec) Marshal(ctx context.Context, msg remote.Message, out remote.ByteBuffer) error {
	svcDsc, ok := c.svcDsc.Load().(gproto.ServiceDescriptor)
	if !ok {
		return perrors.NewProtocolErrorWithMsg("failed to get ServiceDescriptor")
	}

	methodName := msg.RPCInfo().Invocation().MethodName()
	fnDesc := svcDsc.FindMethodByName(methodName)
	if fnDesc == nil {
		return perrors.NewProtocolErrorWithMsg("empty methodName in protobuf Marshal")
	}

	// Get the type of the request protobuf message
	requestType := reflect.TypeOf(fnDesc.GetInputType())
	if requestType.Kind() == reflect.Ptr {
		requestType = requestType.Elem()
	}

	pbReq := fnDesc.AsProto()

	_, err := protob.Marshal(pbReq)
	if err != nil {
		return err
	}

	return c.codec.Marshal(ctx, msg, out)
}

func (c *JSONToProtobufCodec) Unmarshal(ctx context.Context, msg remote.Message, in remote.ByteBuffer) error {
	svcDsc, ok := c.svcDsc.Load().(gproto.ServiceDescriptor)
	if !ok {
		return perrors.NewProtocolErrorWithMsg("failed to get ServiceDescriptor")
	}

	methodName := msg.RPCInfo().Invocation().MethodName()
	fnDesc := svcDsc.FindMethodByName(methodName)
	if fnDesc == nil {
		return perrors.NewProtocolErrorWithMsg("failed to find method")
	}

	// Get the type of the request protobuf message
	requestType := reflect.TypeOf(fnDesc.GetInputType())
	if requestType.Kind() == reflect.Ptr {
		requestType = requestType.Elem()
	}

	pbReq := fnDesc.AsProto()

	// Read the binary data from the input buffer
	data := make([]byte, in.ReadableLen())
	_, err := in.Read(data)
	if err != nil {
		return err
	}

	// Unmarshal the binary data into the request protobuf message
	if err := protob.Unmarshal(data[:in.ReadableLen()], pbReq); err != nil {
		return err
	}

	// Marshal the protobuf message to JSON data
	jsonData, err := json.Marshal(pbReq)
	if err != nil {
		return err
	}

	// Set the JSON data in the message's data
	msg.Data().(WithCodec).SetCodec(jsonData)

	return nil
}

func (c *JSONToProtobufCodec) getMethod(req interface{}, method string) (*Method, error) {
	methodDesc := c.svcDsc.Load().(gproto.ServiceDescriptor).FindMethodByName(method)
	if methodDesc == nil {
		return nil, perrors.NewProtocolErrorWithMsg(fmt.Sprintf("no method: %v in service: %s", method, c.svcDsc.Load().(gproto.ServiceDescriptor).GetFullyQualifiedName()))
	}
	return &Method{method, false}, nil
}

func (c *JSONToProtobufCodec) Name() string {
	return "JSONToProtobuf"
}

func (c *JSONToProtobufCodec) Close() error {
	return nil
}
