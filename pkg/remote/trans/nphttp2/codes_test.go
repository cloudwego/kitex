/*
 * Copyright 2022 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package nphttp2

import (
	"context"
	"testing"

	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/codes"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/status"
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

	// test convertErrorFromGrpcToKitex() grpc err
	permissionDeniedErr := status.Errorf(codes.PermissionDenied, "grpc permission denied")
	err = convertErrorFromGrpcToKitex(permissionDeniedErr)
	test.Assert(t, err != nil, err)

	notFoundErr := status.Errorf(codes.NotFound, "grpc not found")
	err = convertErrorFromGrpcToKitex(notFoundErr)
	test.Assert(t, err != nil, err)
}
