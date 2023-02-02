package loadbalance

import (
	"context"
	"sync/atomic"

	"github.com/cloudwego/kitex/pkg/discovery"
	"github.com/cloudwego/kitex/pkg/klog"
)

var _ Picker = &RoundPicker{}

// RoundPicker .
type RoundPicker struct {
	cursor             int64
	immutableInstances []discovery.Instance
}

func (rp *RoundPicker) Recycle() {}

// Next implements the Picker interface.
func (rp *RoundPicker) Next(ctx context.Context, request interface{}) (ins discovery.Instance) {
	defer func() {
		if ins != nil {
			klog.Infof("KITEX: round picker pick: %s", ins.Address())
		}
	}()

	insList := rp.immutableInstances
	size := len(insList)
	if size == 0 {
		return nil
	}
	idx := atomic.AddInt64(&rp.cursor, 1) % int64(size)
	ins = insList[idx]
	return ins
}
