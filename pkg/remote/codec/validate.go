package codec

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/codec/perrors"
	"github.com/cloudwego/kitex/pkg/remote/transmeta"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"hash/crc32"
	"sync"
)

const (
	maxPayloadChecksumLength = 1024
	PayloadValidatorPrefix   = "PV_"
)

func getValidatorKey(ctx context.Context, p PayloadValidator) string {
	if _, ok := p.(*crcPayloadValidator); ok {
		return p.Key(ctx)
	}
	key := p.Key(ctx)
	return PayloadValidatorPrefix + key
}

func validate(ctx context.Context, message remote.Message, in remote.ByteBuffer, p PayloadValidator) error {
	key := getValidatorKey(ctx, p)
	strInfo := message.TransInfo().TransStrInfo()
	if strInfo == nil {
		return nil
	}
	expectedValue := strInfo[key]
	payloadLen := message.PayloadLen() // total length
	payload, err := in.Peek(payloadLen)
	if err != nil {
		return err
	}
	pass, err := p.Validate(ctx, expectedValue, payload)
	if err != nil {
		return err
	}
	if !pass {
		return perrors.NewProtocolErrorWithType(perrors.InvalidData, "not pass")
	}
	return nil
}

func fillRPCInfo(message remote.Message) {
	if ri := message.RPCInfo(); ri != nil {
		transInfo := message.TransInfo()
		intInfo := transInfo.TransIntInfo()
		from := rpcinfo.AsMutableEndpointInfo(ri.From())
		if from != nil {
			if v := intInfo[transmeta.FromService]; v != "" {
				from.SetServiceName(v)
			}
			if v := intInfo[transmeta.FromMethod]; v != "" {
				from.SetMethod(v)
			}
		}
		to := rpcinfo.AsMutableEndpointInfo(ri.To())
		if to != nil {
			if v := intInfo[transmeta.ToMethod]; v != "" {
				to.SetMethod(v)
			}
			if v := intInfo[transmeta.ToService]; v != "" {
				to.SetServiceName(v)
			}
		}
	}
}

// PayloadValidator is the interface for validating the payload of RPC requests, which allows customized Checksum function.
type PayloadValidator interface {
	// Key returns a key for your validator, which will be the key in ttheader
	Key(ctx context.Context) string

	// Generate generates the checksum of the payload.
	// The value will not be set to the request header if need is false.
	// DO NOT modify the input payload since it might be obtained by nocopy API from the underlying buffer.
	Generate(ctx context.Context, outboundPayload []byte) (need bool, value string, err error)

	// Validate validates the input payload with the attached checksum.
	// Return pass if validation succeed, or return error.
	Validate(ctx context.Context, expectedValue string, inboundPayload []byte) (pass bool, err error)
}

// NewCRC32PayloadValidator returns a new crcPayloadValidator
func NewCRC32PayloadValidator() PayloadValidator {
	crc32TableOnce.Do(func() {
		crc32cTable = crc32.MakeTable(crc32.Castagnoli)
	})
	return &crcPayloadValidator{}
}

type crcPayloadValidator struct{}

var _ PayloadValidator = &crcPayloadValidator{}

// TODO: 2d slice
func (p *crcPayloadValidator) Key(ctx context.Context) string {
	return transmeta.HeaderCRC32C
}

func (p *crcPayloadValidator) Checksum(ctx context.Context, payload []byte) (value string, err error) {
	return getCRC32C([][]byte{payload}), nil
}

func (p *crcPayloadValidator) Generate(ctx context.Context, outPayload []byte) (need bool, value string, err error) {
	return true, getCRC32C([][]byte{outPayload}), nil
}

func (p *crcPayloadValidator) Validate(ctx context.Context, expectedValue string, inputPayload []byte) (pass bool, err error) {
	_, realValue, err := p.Generate(ctx, inputPayload)
	if err != nil {
		return false, err
	}
	if realValue != expectedValue {
		return false, perrors.NewProtocolErrorWithType(perrors.InvalidData, expectedValue)
	}
	return true, nil
}

// crc32cTable is used for crc32c check
var (
	crc32cTable    *crc32.Table
	crc32TableOnce sync.Once
)

// getCRC32C calculates the crc32c checksum of the input bytes.
// the checksum will be converted into big-endian format and encoded into hex string.
func getCRC32C(payload [][]byte) string {
	if crc32cTable == nil {
		return ""
	}
	csb := make([]byte, Size32)
	var checksum uint32
	for i := 0; i < len(payload); i++ {
		checksum = crc32.Update(checksum, crc32cTable, payload[i])
	}
	binary.BigEndian.PutUint32(csb, checksum)
	return hex.EncodeToString(csb)
}

// flatten2DSlice converts 2d slice to 1d.
// total length should be provided.
func flatten2DSlice(b2 [][]byte, length int) []byte {
	b1 := make([]byte, length)
	off := 0
	for i := 0; i < len(b2); i++ {
		off += copy(b1[off:], b2[i])
	}
	return b1
}
