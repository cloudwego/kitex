package thrift

import (
	"context"
	"fmt"
	"math"

	"github.com/cloudwego/gopkg/protocol/thrift"
	"github.com/cloudwego/kitex/pkg/generic/descriptor"
)

type thriftWriter func(ctx context.Context, out *thrift.BufferWriter, val interface{}) error

func nopWriter(ctx context.Context, out *thrift.BufferWriter, val interface{}) error {
	return nil
}

var (
	_ thriftWriter = nopWriter
	_ thriftWriter = voidWriter
)

var writeFunctions = [descriptor.TYPE_UPPERBOUND]thriftWriter{
	nopWriter,    // STOP = 0
	voidWriter,   // VOID = 1
	boolWriter,   // BOOL = 2
	byteWriter,   // BYTE/I08 = 3
	doubleWriter, // DOUBLE = 4
	nopWriter,    // nothing, index = 5
	int16Writer,  // I16 = 6
	nopWriter,    // nothing, index = 7
	int32Writer,  // I32 = 8
	nopWriter,    // nothing, index = 9
	int64Writer,  // I64 = 10
	stringWriter, // STRING = 11
}

func voidWriter(ctx context.Context, out *thrift.BufferWriter, val interface{}) error {
	panic("todo")
}

func boolWriter(ctx context.Context, out *thrift.BufferWriter, val interface{}) error {
	b, ok := val.(bool)
	if !ok {
		return fmt.Errorf("val must be bool, got %T", val)
	}
	return out.WriteBool(b)
}

func byteWriter(ctx context.Context, out *thrift.BufferWriter, val interface{}) error {
	switch b := val.(type) {
	case int8:
		return out.WriteByte(b)
	case uint8:
		return out.WriteByte(int8(b))
	}

	// slow path
	if b, ok := asInt64(val); ok {
		if b <= math.MaxInt8 && b >= math.MinInt8 {
			return out.WriteByte(int8(b))
		}
		return fmt.Errorf("val[%d] is out of range (int8)", b)
	}
	return fmt.Errorf("val must be int, got %T", val)
}

func doubleWriter(ctx context.Context, out *thrift.BufferWriter, val interface{}) error {
	b, ok := asFloat64(val)
	if ok {
		return out.WriteDouble(b)
	}
	return fmt.Errorf("val must be float64, got %T", val)
}

func int16Writer(ctx context.Context, out *thrift.BufferWriter, val interface{}) error {
	if b, ok := val.(int16); ok {
		return out.WriteI16(b)
	}
	if b, ok := asInt64(val); ok {
		if b <= math.MaxInt16 && b >= math.MinInt16 {
			return out.WriteI16(int16(b))
		}
		return fmt.Errorf("val[%d] is out of range (int16)", b)
	}
	return fmt.Errorf("val must be int16, got %T", val)
}

func int32Writer(ctx context.Context, out *thrift.BufferWriter, val interface{}) error {
	if b, ok := val.(int32); ok {
		return out.WriteI32(b)
	}
	if b, ok := asInt64(val); ok {
		if b <= math.MaxInt32 && b >= math.MinInt32 {
			return out.WriteI32(int32(b))
		}
		return fmt.Errorf("val[%d] is out of range (int32)", b)
	}
	return fmt.Errorf("val must be int32, got %T", val)
}

func int64Writer(ctx context.Context, out *thrift.BufferWriter, val interface{}) error {
	if b, ok := val.(int64); ok {
		return out.WriteI64(b)
	}
	if b, ok := asInt64(val); ok {
		return out.WriteI64(b)
	}
	return fmt.Errorf("val must be int64, got %T", val)
}

func stringWriter(ctx context.Context, out *thrift.BufferWriter, val interface{}) error {
	if b, ok := val.(string); ok {
		return out.WriteString(b)
	}
	return fmt.Errorf("val must be string, got %T", val)
}
