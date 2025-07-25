/*
 * Copyright 2024 CloudWeGo Authors
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

package ttstream

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/cloudwego/gopkg/protocol/thrift"

	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/kerrors"
)

func TestErrors(t *testing.T) {
	causeErr := fmt.Errorf("test1")
	newErr := errIllegalFrame.WithCause(causeErr)
	test.Assert(t, errors.Is(newErr, errIllegalFrame), newErr)
	test.Assert(t, errors.Is(newErr, kerrors.ErrStreamingProtocol), newErr)
	test.Assert(t, strings.Contains(newErr.Error(), errIllegalFrame.Error()))
	test.Assert(t, strings.Contains(newErr.Error(), causeErr.Error()))

	appErr := errApplicationException.WithCause(causeErr)
	test.Assert(t, errors.Is(appErr, errApplicationException), appErr)
	test.Assert(t, !errors.Is(appErr, kerrors.ErrStreamingProtocol), appErr)
	test.Assert(t, strings.Contains(appErr.Error(), errApplicationException.Error()))
	test.Assert(t, strings.Contains(appErr.Error(), causeErr.Error()))
}

func TestCommonParentKerror(t *testing.T) {
	errs := []error{
		errUnexpectedHeader,
		errIllegalBizErr,
		errIllegalFrame,
		errIllegalOperation,
		errTransport,
	}
	for _, err := range errs {
		test.Assert(t, errors.Is(err, kerrors.ErrStreamingProtocol), err)
	}
	test.Assert(t, !errors.Is(errApplicationException, kerrors.ErrStreamingProtocol))
}

func TestGetTypeId(t *testing.T) {
	exception := thrift.NewApplicationException(1000, "test")
	normalErr := errors.New("test")
	testcases := []struct {
		err          error
		expectTypeId int32
	}{
		{err: errApplicationException, expectTypeId: 12001},
		{err: errUnexpectedHeader, expectTypeId: 12002},
		{err: errIllegalBizErr, expectTypeId: 12003},
		{err: errIllegalFrame, expectTypeId: 12004},
		{err: errIllegalOperation, expectTypeId: 12005},
		{err: errTransport, expectTypeId: 12006},
		{err: errApplicationException.WithCause(exception), expectTypeId: 1000},
		{err: errApplicationException.WithCause(normalErr), expectTypeId: 12001},
	}

	for _, testcase := range testcases {
		errWithTypeId, ok := testcase.err.(tException)
		test.Assert(t, ok)
		test.Assert(t, errWithTypeId.TypeId() == testcase.expectTypeId, errWithTypeId)
	}
}

func TestCanceledException(t *testing.T) {
	t.Run("biz cancel", func(t *testing.T) {
		bizCancelEx := errBizCancel.NewBuilder().WithSide(clientTransport)
		t.Log(bizCancelEx)
		test.Assert(t, errors.Is(bizCancelEx, kerrors.ErrStreamingCanceled))
		test.Assert(t, errors.Is(bizCancelEx, errBizCancel))
	})

	t.Run("downstream cancel", func(t *testing.T) {
		ex0 := errDownstreamCancel.NewBuilder().WithSide(clientTransport).WithTriggeredBy("Mesh Egress").WithCauseAndTypeId(errors.New("mesh timeout"), 1204)
		t.Log(ex0)
		test.Assert(t, errors.Is(ex0, kerrors.ErrStreamingCanceled))
		test.Assert(t, errors.Is(ex0, errDownstreamCancel))
	})

	t.Run("upstream cancel", func(t *testing.T) {
		bizCancelEx := errBizCancel.NewBuilder().WithSide(clientTransport)
		ex0 := errUpstreamCancel.NewBuilder().WithSide(clientTransport).WithTriggeredBy("p.s.m").WithCauseAndTypeId(thrift.NewApplicationException(bizCancelEx.typeId, bizCancelEx.message), 9999)
		t.Log(ex0)
		test.Assert(t, errors.Is(ex0, kerrors.ErrStreamingCanceled))
		test.Assert(t, errors.Is(ex0, errUpstreamCancel))
		ex1 := errUpstreamCancel.NewBuilder().WithSide(clientTransport).WithTriggeredBy("Mesh Ingress").WithCauseAndTypeId(errors.New("mesh timeout"), 1204)
		t.Log(ex1)
		test.Assert(t, errors.Is(ex1, kerrors.ErrStreamingCanceled))
		test.Assert(t, errors.Is(ex1, errUpstreamCancel))
		ex2 := errUpstreamCancel.NewBuilder().WithSide(clientTransport).WithTriggeredBy("p.s.m").WithCauseAndTypeId(errors.New("doubao cancel with code"), 9999)
		t.Log(ex2)
		test.Assert(t, errors.Is(ex2, kerrors.ErrStreamingCanceled))
		test.Assert(t, errors.Is(ex2, errUpstreamCancel))
		ex3 := errUpstreamCancel.NewBuilder().WithSide(serverTransport).WithTriggeredBy("p.s.m").WithCauseAndTypeId(thrift.NewApplicationException(bizCancelEx.typeId, bizCancelEx.message), bizCancelEx.typeId)
		t.Log(ex3)
		test.Assert(t, errors.Is(ex3, kerrors.ErrStreamingCanceled))
		test.Assert(t, errors.Is(ex3, errUpstreamCancel))
		ex4 := errUpstreamCancel.NewBuilder().WithSide(serverTransport).WithTriggeredBy("AGW").WithCause(errors.New("AGW internal error"))
		t.Log(ex4)
		test.Assert(t, errors.Is(ex4, kerrors.ErrStreamingCanceled))
		test.Assert(t, errors.Is(ex4, errUpstreamCancel))
	})
}

func TestTimeoutException(t *testing.T) {
	t.Run("biz timeout", func(t *testing.T) {
		cliEx := errBizTimeout.NewBuilder().WithSide(clientTransport)
		t.Log(cliEx)
		test.Assert(t, errors.Is(cliEx, kerrors.ErrStreamingTimeout))
		test.Assert(t, errors.Is(cliEx, errBizTimeout))

		srvEx := errBizTimeout.NewBuilder().WithTriggeredBy("p.s.m").WithSide(serverTransport)
		t.Log(srvEx)
		test.Assert(t, errors.Is(srvEx, kerrors.ErrStreamingTimeout))
		test.Assert(t, errors.Is(srvEx, errBizTimeout))
	})

	t.Run("transmission timeout", func(t *testing.T) {

	})
}
