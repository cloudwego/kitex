package codec

import (
	"context"
	"github.com/bytedance/gopkg/util/xxhash3"
	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/remote"
	"strconv"
	"testing"
)

var _ PayloadValidator = &mockPayloadValidator{}

type mockPayloadValidator struct {
}

const (
	mockExceedLimitKey = "mockExceedLimit"
	mockPreprocessKey  = "key"
)

func (m *mockPayloadValidator) Key(ctx context.Context) string {
	return "mockValidator"
}

func (m *mockPayloadValidator) Generate(ctx context.Context, outPayload []byte) (need bool, value string, err error) {
	if l := ctx.Value(mockExceedLimitKey); l != nil {
		// return value with length exceeding the limit
		return true, string(make([]byte, maxPayloadChecksumLength+1)), nil
	}
	hash := xxhash3.Hash(outPayload)
	return true, strconv.FormatInt(int64(hash), 10), nil
}

func (m *mockPayloadValidator) Validate(ctx context.Context, expectedValue string, inputPayload []byte) (pass bool, err error) {
	_, value, err := m.Generate(ctx, inputPayload)
	if err != nil {
		return false, err
	}
	return value == expectedValue, nil
}

func (m *mockPayloadValidator) ProcessBeforeValidate(ctx context.Context, message remote.Message) (context.Context, error) {
	ctx = context.WithValue(ctx, mockPreprocessKey, true)
	return ctx, nil
}

func TestPayloadValidator(t *testing.T) {
	p := &mockPayloadValidator{}
	payload := make([]byte, 0)

	need, value, err := p.Generate(context.Background(), payload)
	test.Assert(t, err == nil, err)
	test.Assert(t, need)

	ctx, err := p.ProcessBeforeValidate(context.Background(), nil)
	test.Assert(t, err == nil, err)
	test.Assert(t, ctx.Value(mockPreprocessKey) == true)

	pass, err := p.Validate(ctx, value, payload)
	test.Assert(t, err == nil, err)
	test.Assert(t, pass, true)
}
