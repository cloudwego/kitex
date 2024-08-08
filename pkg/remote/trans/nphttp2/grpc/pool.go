package grpc

import "sync"

var poolDataFrame = sync.Pool{
	New: func() interface{} {
		return &dataFrame{}
	},
}
