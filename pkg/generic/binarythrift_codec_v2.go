package generic

import (
	"github.com/cloudwego/kitex/pkg/generic/thrift"
)

type binaryThriftCodecV2 struct {
	svcName      string
	readerWriter *thrift.RawReaderWriter
}

func newBinaryThriftCodecV2(svcName string) *binaryThriftCodecV2 {
	return &binaryThriftCodecV2{
		svcName:      svcName,
		readerWriter: thrift.NewRawReaderWriter(),
	}
}

func (c *binaryThriftCodecV2) Name() string {
	return "BinaryThriftV2"
}

func (c *binaryThriftCodecV2) getMessageReaderWriter() interface{} {
	return c.readerWriter
}
