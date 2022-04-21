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
	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/codes"
	spb "google.golang.org/genproto/googleapis/rpc/status"
	"testing"
)

func TestStatusOk(t *testing.T) {

	statusMsg := "test"
	statusOk := Newf(codes.OK, statusMsg)
	test.Assert(t, statusOk.Code() == codes.OK)
	test.Assert(t, statusOk.Message() == statusMsg)
	test.Assert(t, statusOk.Err() == nil)

	details, err := statusOk.WithDetails()
	test.Assert(t, err != nil, err)
	test.Assert(t, details == nil)

	statusOk.Details()

	statusEmpty := Status{}
	test.Assert(t, statusEmpty.Code() == codes.OK)
	test.Assert(t, statusEmpty.Message() == "")

	// error test
	notFoundErr := Errorf(codes.NotFound, statusMsg)
	statusErr, ok := FromError(notFoundErr)
	test.Assert(t, ok)
	test.Assert(t, statusErr.Code() == codes.NotFound)

}

func TestError(t *testing.T) {
	testErr := fmt.Errorf("status unit test error")

	errOutput := "rpc error: code = 1 desc = test err"

	s := new(spb.Status)
	s.Code = 1
	s.Message = "test err"

	er := &Error{s}
	test.Assert(t, er.Error() == errOutput, testErr)

	status := er.GRPCStatus()
	test.Assert(t, status.Message() == s.Message, testErr)

	er.Is(context.Canceled)
}

func TestProto(t *testing.T) {
	//testErr := fmt.Errorf("status unit test error")
	//
	//s := &spb.Status{}
	//proto := status.FromProto(s)
	//test.Assert(t, proto != nil, testErr)

}

func TestFromContextError(t *testing.T) {
	errDdl := context.DeadlineExceeded
	errCanceled := context.Canceled
	errUnknow := fmt.Errorf("unknown error")

	testErr := fmt.Errorf("status unit test error")

	test.Assert(t, FromContextError(nil) == nil, testErr)
	test.Assert(t, FromContextError(errDdl).Code() == codes.DeadlineExceeded, testErr)
	test.Assert(t, FromContextError(errCanceled).Code() == codes.Canceled, testErr)
	test.Assert(t, FromContextError(errUnknow).Code() == codes.Unknown, testErr)

	Code(errUnknow)
	Convert(errUnknow)
}
