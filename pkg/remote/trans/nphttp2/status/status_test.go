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
	streaming_types "github.com/cloudwego/kitex/pkg/streaming/types"
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

func TestTimeoutStatus(t *testing.T) {
	t.Run("stream timeout status", func(t *testing.T) {
		statusMsg := "stream timeout exceeded"
		status := NewTimeoutStatus(codes.DeadlineExceeded, statusMsg, streaming_types.StreamTimeout)
		test.Assert(t, status != nil)
		test.Assert(t, status.Code() == codes.DeadlineExceeded)
		test.Assert(t, status.Message() == statusMsg)
		test.Assert(t, status.TimeoutType() == streaming_types.StreamTimeout)

		err := status.Err()
		test.Assert(t, err != nil)
		statusErr, ok := err.(*Error)
		test.Assert(t, ok)
		test.Assert(t, statusErr.timeoutType == streaming_types.StreamTimeout)

		recoveredStatus := statusErr.GRPCStatus()
		test.Assert(t, recoveredStatus.Code() == codes.DeadlineExceeded)
		test.Assert(t, recoveredStatus.Message() == statusMsg)
	})
	t.Run("stream recv timeout status", func(t *testing.T) {
		statusMsg := "stream recv timeout exceeded"
		status := NewTimeoutStatus(codes.DeadlineExceeded, statusMsg, streaming_types.StreamRecvTimeout)
		test.Assert(t, status != nil)
		test.Assert(t, status.Code() == codes.DeadlineExceeded)
		test.Assert(t, status.Message() == statusMsg)
		test.Assert(t, status.TimeoutType() == streaming_types.StreamRecvTimeout)

		err := status.Err()
		test.Assert(t, err != nil)
		statusErr, ok := err.(*Error)
		test.Assert(t, ok)
		test.Assert(t, statusErr.timeoutType == streaming_types.StreamRecvTimeout)
	})
	t.Run("stream send timeout status", func(t *testing.T) {
		statusMsg := "stream send timeout exceeded"
		status := NewTimeoutStatus(codes.DeadlineExceeded, statusMsg, streaming_types.StreamSendTimeout)
		test.Assert(t, status != nil)
		test.Assert(t, status.Code() == codes.DeadlineExceeded)
		test.Assert(t, status.Message() == statusMsg)
		test.Assert(t, status.TimeoutType() == streaming_types.StreamSendTimeout)

		err := status.Err()
		test.Assert(t, err != nil)
		statusErr, ok := err.(*Error)
		test.Assert(t, ok)
		test.Assert(t, statusErr.timeoutType == streaming_types.StreamSendTimeout)
	})
	t.Run("nil status timeout type", func(t *testing.T) {
		var nilStatus *Status
		test.Assert(t, nilStatus.TimeoutType() == 0)
		test.Assert(t, nilStatus.Code() == codes.OK)
		test.Assert(t, nilStatus.Message() == "")
	})
}

func TestCancelStatus(t *testing.T) {
	t.Run("local cascaded cancel status", func(t *testing.T) {
		statusMsg := "canceled by handler"
		status := NewCancelStatus(codes.Canceled, statusMsg, streaming_types.LocalCascadedCancel)
		test.Assert(t, status != nil)
		test.Assert(t, status.Code() == codes.Canceled)
		test.Assert(t, status.Message() == statusMsg)
		test.Assert(t, status.CancelType() == streaming_types.LocalCascadedCancel)

		err := status.Err()
		test.Assert(t, err != nil)
		statusErr, ok := err.(*Error)
		test.Assert(t, ok)
		test.Assert(t, statusErr.cancelType == streaming_types.LocalCascadedCancel)

		recoveredStatus := statusErr.GRPCStatus()
		test.Assert(t, recoveredStatus.Code() == codes.Canceled)
		test.Assert(t, recoveredStatus.Message() == statusMsg)
		test.Assert(t, recoveredStatus.CancelType() == streaming_types.LocalCascadedCancel)
	})

	t.Run("remote cascaded cancel status", func(t *testing.T) {
		statusMsg := "canceled by remote service"
		status := NewCancelStatus(codes.Canceled, statusMsg, streaming_types.RemoteCascadedCancel)
		test.Assert(t, status != nil)
		test.Assert(t, status.Code() == codes.Canceled)
		test.Assert(t, status.Message() == statusMsg)
		test.Assert(t, status.CancelType() == streaming_types.RemoteCascadedCancel)

		err := status.Err()
		test.Assert(t, err != nil)
		statusErr, ok := err.(*Error)
		test.Assert(t, ok)
		test.Assert(t, statusErr.cancelType == streaming_types.RemoteCascadedCancel)
	})

	t.Run("active cancel status", func(t *testing.T) {
		statusMsg := "actively canceled"
		status := NewCancelStatus(codes.Canceled, statusMsg, streaming_types.ActiveCancel)
		test.Assert(t, status != nil)
		test.Assert(t, status.Code() == codes.Canceled)
		test.Assert(t, status.Message() == statusMsg)
		test.Assert(t, status.CancelType() == streaming_types.ActiveCancel)

		err := status.Err()
		test.Assert(t, err != nil)
		statusErr, ok := err.(*Error)
		test.Assert(t, ok)
		test.Assert(t, statusErr.cancelType == streaming_types.ActiveCancel)
	})

	t.Run("nil status cancel type", func(t *testing.T) {
		var nilStatus *Status
		test.Assert(t, nilStatus.CancelType() == 0)
		test.Assert(t, nilStatus.Code() == codes.OK)
		test.Assert(t, nilStatus.Message() == "")
	})

	t.Run("OK status with cancel type should return nil error", func(t *testing.T) {
		status := NewCancelStatus(codes.OK, "ok message", streaming_types.ActiveCancel)
		test.Assert(t, status.Err() == nil)
	})
}

func TestRemoteBusinessStatus(t *testing.T) {
	t.Run("remote business status with explicit flag", func(t *testing.T) {
		statusMsg := "business logic error"
		status := NewRemoteBusinessStatus(codes.InvalidArgument, statusMsg)
		test.Assert(t, status != nil)
		test.Assert(t, status.Code() == codes.InvalidArgument)
		test.Assert(t, status.Message() == statusMsg)
		test.Assert(t, status.IsFromRemoteBusiness())

		// Test that it's not recognized as timeout or cancel
		test.Assert(t, status.TimeoutType() == 0)
		test.Assert(t, status.CancelType() == 0)
	})

	t.Run("status with [biz error] suffix", func(t *testing.T) {
		statusMsg := "validation failed [biz error]"
		status := New(codes.InvalidArgument, statusMsg)
		test.Assert(t, status != nil)
		test.Assert(t, status.Code() == codes.InvalidArgument)
		test.Assert(t, status.Message() == statusMsg)
		test.Assert(t, status.IsFromRemoteBusiness(), "should detect [biz error] suffix")
	})

	t.Run("regular status without biz error marker", func(t *testing.T) {
		statusMsg := "internal error"
		status := New(codes.Internal, statusMsg)
		test.Assert(t, status != nil)
		test.Assert(t, !status.IsFromRemoteBusiness(), "should not be recognized as business error")
	})

	t.Run("nil status is not from business", func(t *testing.T) {
		var nilStatus *Status
		test.Assert(t, !nilStatus.IsFromRemoteBusiness())
	})

	t.Run("FromError preserves business status", func(t *testing.T) {
		originalStatus := NewRemoteBusinessStatus(codes.FailedPrecondition, "precondition failed")
		test.Assert(t, originalStatus.IsFromRemoteBusiness())

		err := originalStatus.Err()
		test.Assert(t, err != nil)

		// Now fromRemoteBusiness flag is preserved through Error
		recoveredStatus, ok := FromError(err)
		test.Assert(t, ok)
		test.Assert(t, recoveredStatus != nil)
		test.Assert(t, recoveredStatus.Code() == codes.FailedPrecondition)
		test.Assert(t, recoveredStatus.IsFromRemoteBusiness(), "fromRemoteBusiness should be preserved")
	})

	t.Run("Error preserves business flag through GRPCStatus", func(t *testing.T) {
		status := NewRemoteBusinessStatus(codes.InvalidArgument, "validation error")
		err := status.Err()
		test.Assert(t, err != nil)

		statusErr, ok := err.(*Error)
		test.Assert(t, ok)

		// GRPCStatus should preserve the flag
		recoveredStatus := statusErr.GRPCStatus()
		test.Assert(t, recoveredStatus.IsFromRemoteBusiness())
		test.Assert(t, recoveredStatus.Code() == codes.InvalidArgument)
	})

	t.Run("SetFromRemoteBusiness sets flag to true", func(t *testing.T) {
		status := New(codes.InvalidArgument, "some error")
		test.Assert(t, !status.IsFromRemoteBusiness(), "initially should not be business error")

		status.SetFromRemoteBusiness(true)
		test.Assert(t, status.IsFromRemoteBusiness(), "should be business error after setting")
	})

	t.Run("SetFromRemoteBusiness sets flag to false", func(t *testing.T) {
		status := NewRemoteBusinessStatus(codes.InvalidArgument, "business error")
		test.Assert(t, status.IsFromRemoteBusiness(), "initially should be business error")

		status.SetFromRemoteBusiness(false)
		test.Assert(t, !status.IsFromRemoteBusiness(), "should not be business error after clearing")
	})

	t.Run("SetFromRemoteBusiness on nil status does nothing", func(t *testing.T) {
		var nilStatus *Status
		// Should not panic
		nilStatus.SetFromRemoteBusiness(true)
		test.Assert(t, !nilStatus.IsFromRemoteBusiness())
	})

	t.Run("SetFromRemoteBusiness true has higher priority than suffix", func(t *testing.T) {
		// Status without [biz error] suffix
		status := New(codes.InvalidArgument, "regular error")
		test.Assert(t, !status.IsFromRemoteBusiness(), "should not detect as business error")

		// Set to true should mark as business error even without suffix
		status.SetFromRemoteBusiness(true)
		test.Assert(t, status.IsFromRemoteBusiness(), "explicit flag should work without suffix")
	})

	t.Run("SetFromRemoteBusiness false still checks suffix", func(t *testing.T) {
		// Status with [biz error] suffix
		status := New(codes.InvalidArgument, "error [biz error]")
		test.Assert(t, status.IsFromRemoteBusiness(), "should detect suffix")

		// Set to false doesn't override suffix detection (suffix is fallback)
		status.SetFromRemoteBusiness(false)
		test.Assert(t, status.IsFromRemoteBusiness(), "suffix detection still works when flag is false")
	})

	t.Run("SetFromRemoteBusiness persists through Err and GRPCStatus", func(t *testing.T) {
		status := New(codes.InvalidArgument, "error")
		status.SetFromRemoteBusiness(true)

		err := status.Err()
		recoveredStatus, ok := FromError(err)
		test.Assert(t, ok)
		test.Assert(t, recoveredStatus.IsFromRemoteBusiness(), "flag should persist")
	})
}

func TestAppendMessage(t *testing.T) {
	t.Run("append message to normal status", func(t *testing.T) {
		originalMsg := "original error"
		extraMsg := "additional context"
		status := New(codes.Internal, originalMsg)

		updatedStatus := status.AppendMessage(extraMsg)
		test.Assert(t, updatedStatus != nil)
		test.Assert(t, updatedStatus.Message() == "original error additional context")
		test.Assert(t, updatedStatus.Code() == codes.Internal)
	})

	t.Run("append empty message does nothing", func(t *testing.T) {
		originalMsg := "error message"
		status := New(codes.Internal, originalMsg)

		updatedStatus := status.AppendMessage("")
		test.Assert(t, updatedStatus.Message() == originalMsg)
	})

	t.Run("append message to nil status returns nil", func(t *testing.T) {
		var nilStatus *Status
		updatedStatus := nilStatus.AppendMessage("extra message")
		test.Assert(t, updatedStatus == nil)
	})

	t.Run("append message preserves cancel type", func(t *testing.T) {
		status := NewCancelStatus(codes.Canceled, "canceled", streaming_types.ActiveCancel)
		updatedStatus := status.AppendMessage("by user")

		test.Assert(t, updatedStatus.Message() == "canceled by user")
		test.Assert(t, updatedStatus.CancelType() == streaming_types.ActiveCancel)
	})

	t.Run("append message preserves timeout type", func(t *testing.T) {
		status := NewTimeoutStatus(codes.DeadlineExceeded, "timeout", streaming_types.StreamTimeout)
		updatedStatus := status.AppendMessage("exceeded")

		test.Assert(t, updatedStatus.Message() == "timeout exceeded")
		test.Assert(t, updatedStatus.TimeoutType() == streaming_types.StreamTimeout)
	})

	t.Run("append message preserves business flag", func(t *testing.T) {
		status := NewRemoteBusinessStatus(codes.InvalidArgument, "invalid input")
		updatedStatus := status.AppendMessage("from service A")

		test.Assert(t, updatedStatus.Message() == "invalid input from service A")
		test.Assert(t, updatedStatus.IsFromRemoteBusiness())
	})
}

func TestWithDetails(t *testing.T) {
	t.Run("WithDetails preserves timeout type", func(t *testing.T) {
		status := NewTimeoutStatus(codes.DeadlineExceeded, "timeout", streaming_types.StreamTimeout)
		test.Assert(t, status.TimeoutType() == streaming_types.StreamTimeout)

		statusWithDetails, err := status.WithDetails(&MockReq{})
		test.Assert(t, err == nil)
		test.Assert(t, statusWithDetails != nil)
		test.Assert(t, statusWithDetails.TimeoutType() == streaming_types.StreamTimeout, "timeout type should be preserved")
		test.Assert(t, len(statusWithDetails.Details()) == 1)
	})

	t.Run("WithDetails preserves cancel type", func(t *testing.T) {
		status := NewCancelStatus(codes.Canceled, "canceled", streaming_types.RemoteCascadedCancel)
		test.Assert(t, status.CancelType() == streaming_types.RemoteCascadedCancel)

		statusWithDetails, err := status.WithDetails(&MockReq{})
		test.Assert(t, err == nil)
		test.Assert(t, statusWithDetails != nil)
		test.Assert(t, statusWithDetails.CancelType() == streaming_types.RemoteCascadedCancel, "cancel type should be preserved")
	})

	t.Run("WithDetails preserves business flag", func(t *testing.T) {
		status := NewRemoteBusinessStatus(codes.InvalidArgument, "business error")
		test.Assert(t, status.IsFromRemoteBusiness())

		statusWithDetails, err := status.WithDetails(&MockReq{})
		test.Assert(t, err == nil)
		test.Assert(t, statusWithDetails != nil)
		test.Assert(t, statusWithDetails.IsFromRemoteBusiness(), "business flag should be preserved")
	})

	t.Run("WithDetails on OK status returns error", func(t *testing.T) {
		status := New(codes.OK, "ok")
		statusWithDetails, err := status.WithDetails(&MockReq{})
		test.Assert(t, err != nil)
		test.Assert(t, statusWithDetails == nil)
	})
}

func TestCombinedScenarios(t *testing.T) {
	t.Run("timeout status should not be recognized as cancel", func(t *testing.T) {
		timeoutStatus := NewTimeoutStatus(codes.DeadlineExceeded, "timeout", streaming_types.StreamTimeout)
		test.Assert(t, timeoutStatus.TimeoutType() != 0)
		test.Assert(t, timeoutStatus.CancelType() == 0)
		test.Assert(t, !timeoutStatus.IsFromRemoteBusiness())
	})

	t.Run("cancel status should not be recognized as timeout", func(t *testing.T) {
		cancelStatus := NewCancelStatus(codes.Canceled, "canceled", streaming_types.ActiveCancel)
		test.Assert(t, cancelStatus.CancelType() != 0)
		test.Assert(t, cancelStatus.TimeoutType() == 0)
		test.Assert(t, !cancelStatus.IsFromRemoteBusiness())
	})

	t.Run("business status should not have timeout or cancel type", func(t *testing.T) {
		bizStatus := NewRemoteBusinessStatus(codes.InvalidArgument, "business error")
		test.Assert(t, bizStatus.IsFromRemoteBusiness())
		test.Assert(t, bizStatus.TimeoutType() == 0)
		test.Assert(t, bizStatus.CancelType() == 0)
	})

	t.Run("status with biz error suffix in message", func(t *testing.T) {
		status := New(codes.FailedPrecondition, "custom validation failed [biz error]")
		test.Assert(t, status.IsFromRemoteBusiness(), "should detect [biz error] suffix")

		// After appending message, [biz error] is no longer at the end
		// So IsFromRemoteBusiness will return false (unless using explicit flag)
		updated := status.AppendMessage("with details")
		test.Assert(t, !updated.IsFromRemoteBusiness(), "[biz error] is no longer a suffix after append")
		test.Assert(t, updated.Message() == "custom validation failed [biz error] with details")
	})
}
