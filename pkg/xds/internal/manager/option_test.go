package manager

import (
	"github.com/cloudwego/kitex/internal/test"
	"testing"
)

func Test_xdsResourceManager_Option(t *testing.T) {
	addr := "localhost:8080"
	opts := []Option{
		{
			F: func(o *Options) {
				o.XDSSvrConfig.SvrAddr = addr
			},
		},
	}
	o := NewOptions(opts)
	test.Assert(t, o.XDSSvrConfig.SvrAddr == addr)
}
