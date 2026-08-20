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

package status

import (
	"context"
	"fmt"
	"testing"

	spb "google.golang.org/genproto/googleapis/rpc/status"

	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/codes"
)

func TestStatus(t *testing.T) {
	// test ok status
	statusMsg := "test"
	statusOk := Newf(codes.OK, "%s", statusMsg)
	test.Assert(t, statusOk.Code() == codes.OK)
	test.Assert(t, statusOk.Message() == statusMsg)
	test.Assert(t, statusOk.Err() == nil)

	details, err := statusOk.WithDetails()
	test.Assert(t, err != nil, err)
	test.Assert(t, details == nil)

	okDetails := statusOk.Details()
	test.Assert(t, len(okDetails) == 0)

	// test empty status
	statusEmpty := Status{}
	test.Assert(t, statusEmpty.Code() == codes.OK)
	test.Assert(t, statusEmpty.Message() == "")
	emptyDetail := statusEmpty.Details()
	test.Assert(t, emptyDetail == nil)

	// test error status
	notFoundErr := Errorf(codes.NotFound, "%s", statusMsg)
	statusErr, ok := FromError(notFoundErr)
	test.Assert(t, ok)
	test.Assert(t, statusErr.Code() == codes.NotFound)

	statusErrWithDetail, err := statusErr.WithDetails(&MockReq{})
	test.Assert(t, err == nil, err)
	notFoundDetails := statusErrWithDetail.Details()
	test.Assert(t, len(notFoundDetails) == 1)

	statusNilErr, ok := FromError(nil)
	test.Assert(t, ok)
	test.Assert(t, statusNilErr == nil)
}

func TestError(t *testing.T) {
	s := new(spb.Status)
	s.Code = 1
	s.Message = "test err"

	er := &Error{e: s}
	test.Assert(t, len(er.Error()) > 0)

	status := er.GRPCStatus()
	test.Assert(t, status.Message() == s.Message)

	is := er.Is(context.Canceled)
	test.Assert(t, !is)

	is = er.Is(er)
	test.Assert(t, is)
}

func TestFromContextError(t *testing.T) {
	errDdl := context.DeadlineExceeded
	errCanceled := context.Canceled
	errUnknown := fmt.Errorf("unknown error")

	testErr := fmt.Errorf("status unit test error")

	test.Assert(t, FromContextError(nil) == nil, testErr)
	test.Assert(t, FromContextError(errDdl).Code() == codes.DeadlineExceeded, testErr)
	test.Assert(t, FromContextError(errCanceled).Code() == codes.Canceled, testErr)
	test.Assert(t, FromContextError(errUnknown).Code() == codes.Unknown, testErr)

	statusCanceled := Convert(context.Canceled)
	test.Assert(t, statusCanceled != nil)

	s := new(spb.Status)
	s.Code = 1
	s.Message = "test err"
	grpcErr := &Error{e: s}
	// grpc err
	codeGrpcErr := Code(grpcErr)
	test.Assert(t, codeGrpcErr == codes.Canceled)

	// non-grpc err
	codeCanceled := Code(errUnknown)
	test.Assert(t, codeCanceled == codes.Unknown)

	// no err
	codeNil := Code(nil)
	test.Assert(t, codeNil == codes.OK)
}

func TestCascadeCancelStatus(t *testing.T) {
	normal := New(codes.Canceled, "normal cancellation")
	unmarkedErr := normal.Err().(*Error)
	markedErr := unmarkedErr.WithCascadeCancel()
	marked := markedErr.GRPCStatus()

	test.Assert(t, !normal.IsCascadeCancel())
	test.Assert(t, marked.IsCascadeCancel())
	test.Assert(t, markedErr.WithCascadeCancel() == markedErr)
	test.Assert(t, markedErr != unmarkedErr)
	test.Assert(t, !unmarkedErr.GRPCStatus().IsCascadeCancel())
	test.Assert(t, marked.Code() == normal.Code())
	test.Assert(t, marked.Message() == normal.Message())

	// non codes.Canceled
	for _, code := range []codes.Code{codes.OK, codes.Unavailable} {
		st := New(code, "not canceled")
		test.Assert(t, !st.IsCascadeCancel())
	}

	// Status -> error -> Status and wrapped error conversion preserve the process-local marker
	err := marked.Err()
	converted, ok := FromError(err)
	test.Assert(t, ok)
	test.Assert(t, converted.IsCascadeCancel())

	converted, ok = FromError(fmt.Errorf("wrapped: %w", err))
	test.Assert(t, ok)
	test.Assert(t, converted.IsCascadeCancel())

	markedErr = err.(*Error)
	test.Assert(t, markedErr.WithCascadeCancel() == markedErr)
	unavailableErr := New(codes.Unavailable, "unavailable").Err().(*Error)
	test.Assert(t, unavailableErr.WithCascadeCancel() == unavailableErr)
	test.Assert(t, !unavailableErr.GRPCStatus().IsCascadeCancel())

	withDetails, err := marked.WithDetails(&MockReq{})
	test.Assert(t, err == nil, err)
	test.Assert(t, withDetails.IsCascadeCancel())
	test.Assert(t, len(withDetails.Details()) == 1)

	// Proto is the wire representation, so a proto round trip intentionally
	// drops the process-local marker.
	test.Assert(t, !FromProto(marked.Proto()).IsCascadeCancel())

	var nilStatus *Status
	test.Assert(t, !nilStatus.IsCascadeCancel())
}
