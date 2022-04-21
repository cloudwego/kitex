package nphttp2

import (
	"context"
	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/codes"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/status"
	"testing"
)

func TestConvertFromKitexToGrpc(t *testing.T) {

	// test convertFromKitexToGrpc() ok status
	status := convertFromKitexToGrpc(nil)
	test.Assert(t, status.Code() == codes.OK)

	// test convertFromKitexToGrpc() kitex err status
	status = convertFromKitexToGrpc(kerrors.ErrACL)
	test.Assert(t, status.Code() == codes.PermissionDenied)

	// test convertFromKitexToGrpc() err status
	status = convertFromKitexToGrpc(context.Canceled)
	test.Assert(t, status.Code() == codes.Internal)

}

func TestConvertErrorFromGrpcToKitex(t *testing.T) {

	// test convertErrorFromGrpcToKitex() no err
	err := convertErrorFromGrpcToKitex(nil)
	test.Assert(t, err == nil, err)

	// test convertErrorFromGrpcToKitex() not grpc err
	err = convertErrorFromGrpcToKitex(context.Canceled)
	test.Assert(t, err == context.Canceled, err)

	//fixme: assert detail

	// test convertErrorFromGrpcToKitex() grpc err
	permissionDeniedErr := status.Errorf(codes.PermissionDenied, "grpc permission denied")
	err = convertErrorFromGrpcToKitex(permissionDeniedErr)
	test.Assert(t, err != nil, err)

	notFoundErr := status.Errorf(codes.NotFound, "grpc not found")
	err = convertErrorFromGrpcToKitex(notFoundErr)
	test.Assert(t, err != nil, err)
}
