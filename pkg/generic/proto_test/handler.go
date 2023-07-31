package main

import (
	"context"
	kitex_gen "github.com/cloudwego/kitex/pkg/generic/proto_test/kitex_gen/kitex_gen"
)

// ExampleServiceImpl implements the last service interface defined in the IDL.
type ExampleServiceImpl struct{}

// Echo implements the ExampleServiceImpl interface.
func (s *ExampleServiceImpl) Echo(ctx context.Context, req *kitex_gen.Message) (resp *kitex_gen.Message, err error) {
	// TODO: Your code here...
	return 
}
