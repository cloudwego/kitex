package grpc

import (
	"github.com/cloudwego/kitex/internal/test"
	"testing"
)

func TestBdp(t *testing.T) {

	bdpEst := &bdpEstimator{
		bdp:               initialWindowSize,
		updateFlowControl: func(n uint32) {},
	}

	size := 100

	//mock data frame receive
	sent := bdpEst.add(uint32(size))
	test.Assert(t, sent)
	bdpEst.timesnap(bdpPing.data)

	// mock bdp send delay
	for i := 0; i < 3; i++ {
		sent = bdpEst.add(uint32(size))
		test.Assert(t, !sent)
	}

	// now receive data
	bdpEst.calculate(bdpPing.data)

	size = 10000
	// calculate multi times
	for c := 0; c < 15; c++ {
		sent = bdpEst.add(uint32(size))
		test.Assert(t, sent)
		bdpEst.timesnap(bdpPing.data)

		//mock the situation that delay is very long and data is very big
		for i := 0; i < 15; i++ {
			sent = bdpEst.add(uint32(size))
			test.Assert(t, !sent)
		}

		// receive and calculate again
		bdpEst.calculate(bdpPing.data)
	}

}
