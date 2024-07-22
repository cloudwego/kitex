package codec

import (
	"context"
	"fmt"
	"github.com/bytedance/gopkg/util/xxhash3"
	"strconv"
)

var _ PayloadValidator = &mockPayloadValidator{}

type mockPayloadValidator struct {
}

func (m *mockPayloadValidator) Key(ctx context.Context) string {
	return "mockValidator"
}

func (m *mockPayloadValidator) Checksum(ctx context.Context, payload []byte) (value string, err error) {
	hash := xxhash3.Hash(payload)
	return strconv.FormatInt(int64(hash), 10), nil
}

func (m *mockPayloadValidator) ErrMsgFunc(ctx context.Context, expected, actual string) string {
	return fmt.Sprintf("mockValidator ErrMsgFunc expected:%s, actual:%s", expected, actual)
}
