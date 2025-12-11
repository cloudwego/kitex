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
	"time"

	"github.com/cloudwego/gopkg/protocol/thrift"

	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/cloudwego/kitex/pkg/remote"
	streaming_types "github.com/cloudwego/kitex/pkg/streaming/types"
)

func TestErrors(t *testing.T) {
	causeErr := fmt.Errorf("test1")
	newErr := errIllegalFrame.newBuilder().withCause(causeErr)
	test.Assert(t, errors.Is(newErr, errIllegalFrame), newErr)
	test.Assert(t, errors.Is(newErr, kerrors.ErrStreamingProtocol), newErr)
	test.Assert(t, strings.Contains(newErr.Error(), causeErr.Error()))

	appErr := errApplicationException.newBuilder().withCause(causeErr)
	test.Assert(t, errors.Is(appErr, errApplicationException), appErr)
	test.Assert(t, !errors.Is(appErr, kerrors.ErrStreamingProtocol), appErr)
	test.Assert(t, strings.Contains(appErr.Error(), causeErr.Error()))

	newExWithNilErr := errIllegalFrame.newBuilder().withCause(nil)
	test.Assert(t, !newExWithNilErr.isCauseSet(), newExWithNilErr)
	test.Assert(t, newExWithNilErr.cause == nil, newExWithNilErr)
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

	// canceled Exception
	errs = []error{
		errBizCancel,
		errBizCancelWithCause,
		errDownstreamCancel,
		errUpstreamCancel,
		errInternalCancel,
	}
	for _, err := range errs {
		test.Assert(t, errors.Is(err, kerrors.ErrStreamingCanceled), err)
	}

	// timeout Exception
	test.Assert(t, errors.Is(errStreamTimeout, kerrors.ErrStreamingTimeout))
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
		{err: errApplicationException.newBuilder().withCauseAndTypeId(exception, 1000), expectTypeId: 1000},
		{err: errApplicationException.newBuilder().withCause(normalErr), expectTypeId: 12001},
	}

	for _, testcase := range testcases {
		errWithTypeId, ok := testcase.err.(*Exception)
		test.Assert(t, ok)
		test.Assert(t, errWithTypeId.TypeId() == testcase.expectTypeId, errWithTypeId)
	}
}

func TestCanceledException(t *testing.T) {
	t.Run("biz cancel", func(t *testing.T) {
		// [ttstream error, code=12007] [client-side stream] user code invoking stream RPC with context processed
		// by context.WithCancel or context.WithTimeout, then invoking cancel() actively
		bizCancelEx := errBizCancel.newBuilder().withSide(clientSide)
		t.Log(bizCancelEx)
		test.Assert(t, errors.Is(bizCancelEx, kerrors.ErrStreamingCanceled))
		test.Assert(t, errors.Is(bizCancelEx, errBizCancel))
	})

	t.Run("downstream cancel", func(t *testing.T) {
		// [ttstream error, code=1111] [client-side stream] [canceled path: Proxy Egress] proxy timeout
		ex0 := errDownstreamCancel.newBuilder().withSide(clientSide).setOrAppendCancelPath("Proxy Egress").withCauseAndTypeId(errors.New("proxy timeout"), 1111)
		t.Log(ex0)
		test.Assert(t, errors.Is(ex0, kerrors.ErrStreamingCanceled))
		test.Assert(t, errors.Is(ex0, errDownstreamCancel))
		test.Assert(t, ex0.TypeId() == 1111, ex0.TypeId())
	})

	t.Run("upstream cancel", func(t *testing.T) {
		bizCancelEx := errBizCancel.newBuilder().withSide(clientSide)
		// [ttstream error, code=12007] [server-side stream] [canceled path: ttstream ServiceA]
		// user code invoking stream RPC with context processed by context.WithCancel or context.WithTimeout, then invoking cancel() actively
		ex0 := errUpstreamCancel.newBuilder().withSide(serverSide).setOrAppendCancelPath("ttstream ServiceA").withCauseAndTypeId(thrift.NewApplicationException(bizCancelEx.TypeId(), bizCancelEx.getMessage()), bizCancelEx.TypeId())
		t.Log(ex0)
		test.Assert(t, errors.Is(ex0, kerrors.ErrStreamingCanceled))
		test.Assert(t, errors.Is(ex0, errUpstreamCancel))
		test.Assert(t, ex0.TypeId() == bizCancelEx.TypeId(), ex0.TypeId())
		// [ttstream error, code=1111] [server-side stream] [canceled path: Proxy Ingress] proxy timeout
		ex1 := errUpstreamCancel.newBuilder().withSide(serverSide).setOrAppendCancelPath("Proxy Ingress").withCauseAndTypeId(errors.New("proxy timeout"), 1111)
		t.Log(ex1)
		test.Assert(t, errors.Is(ex1, kerrors.ErrStreamingCanceled))
		test.Assert(t, errors.Is(ex1, errUpstreamCancel))
		test.Assert(t, ex1.TypeId() == 1111, ex1.TypeId())
		// [ttstream error, code=9999] [server-side stream] [canceled path: ttstream ServiceA] user cancels with code
		ex2 := errUpstreamCancel.newBuilder().withSide(serverSide).setOrAppendCancelPath("ttstream ServiceA").withCauseAndTypeId(errors.New("user cancels with code"), 9999)
		t.Log(ex2)
		test.Assert(t, errors.Is(ex2, kerrors.ErrStreamingCanceled))
		test.Assert(t, errors.Is(ex2, errUpstreamCancel))
		test.Assert(t, ex2.TypeId() == 9999, ex2.TypeId())
		// cascading cancel
		// [ttstream error, code=12007] [client-side stream] [canceled path: ttstream ServiceA -> ttstream ServiceB]
		// user code invoking stream RPC with context processed by context.WithCancel or context.WithTimeout, then invoking cancel() actively
		ex3 := ex0.newBuilder().withSide(clientSide).setOrAppendCancelPath("ttstream ServiceB")
		t.Log(ex3)
		test.Assert(t, errors.Is(ex3, kerrors.ErrStreamingCanceled))
		test.Assert(t, errors.Is(ex3, errUpstreamCancel))
		test.Assert(t, ex3.TypeId() == bizCancelEx.TypeId(), ex3.TypeId())
		// cascading cancel
		// [ttstream error, code=1111] [client-side stream] [canceled path: Proxy Ingress -> ttstream ServiceB] proxy timeout
		ex4 := ex1.newBuilder().withSide(clientSide).setOrAppendCancelPath("ttstream ServiceB")
		t.Log(ex4)
		test.Assert(t, errors.Is(ex4, kerrors.ErrStreamingCanceled))
		test.Assert(t, errors.Is(ex4, errUpstreamCancel))
		// cascading cancel
		// [ttstream error, code=9999] [client-side stream] [canceled path: ttstream ServiceA -> ttstream ServiceB] user cancels with code
		ex5 := ex2.newBuilder().withSide(clientSide).setOrAppendCancelPath("ttstream ServiceB")
		t.Log(ex5)
		test.Assert(t, errors.Is(ex5, kerrors.ErrStreamingCanceled))
		test.Assert(t, errors.Is(ex5, errUpstreamCancel))
	})
}

func TestTimeoutException(t *testing.T) {
	t.Run("stream timeout", func(t *testing.T) {
		// [ttstream error, code=12014] [client-side stream] timeout config: 1s
		clientEx := NewTimeoutException(streaming_types.StreamTimeout, remote.Client, time.Second)
		t.Log(clientEx)
		test.Assert(t, clientEx != nil)
		test.Assert(t, errors.Is(clientEx, kerrors.ErrStreamingTimeout))
		test.Assert(t, errors.Is(clientEx, errStreamTimeout))
		test.Assert(t, clientEx.TypeId() == 12014, clientEx.TypeId())

		// [ttstream error, code=12014] [server-side stream] timeout config: 2s
		serverEx := NewTimeoutException(streaming_types.StreamTimeout, remote.Server, 2*time.Second)
		t.Log(serverEx)
		test.Assert(t, serverEx != nil)
		test.Assert(t, errors.Is(serverEx, kerrors.ErrStreamingTimeout))
		test.Assert(t, errors.Is(serverEx, errStreamTimeout))
		test.Assert(t, serverEx.TypeId() == 12014, serverEx.TypeId())
	})

	t.Run("stream recv timeout", func(t *testing.T) {
		// [ttstream error, code=12015] [client-side stream] timeout config: 500ms
		clientEx := NewTimeoutException(streaming_types.StreamRecvTimeout, remote.Client, 500*time.Millisecond)
		t.Log(clientEx)
		test.Assert(t, clientEx != nil)
		test.Assert(t, errors.Is(clientEx, kerrors.ErrStreamingTimeout))
		test.Assert(t, errors.Is(clientEx, errStreamRecvTimeout))
		test.Assert(t, clientEx.TypeId() == 12015, clientEx.TypeId())

		// [ttstream error, code=12015] [server-side stream] timeout config: 1s
		serverEx := NewTimeoutException(streaming_types.StreamRecvTimeout, remote.Server, time.Second)
		t.Log(serverEx)
		test.Assert(t, serverEx != nil)
		test.Assert(t, errors.Is(serverEx, kerrors.ErrStreamingTimeout))
		test.Assert(t, errors.Is(serverEx, errStreamRecvTimeout))
		test.Assert(t, serverEx.TypeId() == 12015, serverEx.TypeId())
	})

	t.Run("stream send timeout", func(t *testing.T) {
		// [ttstream error, code=12016] [client-side stream] timeout config: 800ms
		clientEx := NewTimeoutException(streaming_types.StreamSendTimeout, remote.Client, 800*time.Millisecond)
		t.Log(clientEx)
		test.Assert(t, clientEx != nil)
		test.Assert(t, errors.Is(clientEx, kerrors.ErrStreamingTimeout))
		test.Assert(t, errors.Is(clientEx, errStreamSendTimeout))
		test.Assert(t, clientEx.TypeId() == 12016, clientEx.TypeId())

		// [ttstream error, code=12016] [server-side stream] timeout config: 1.5s
		serverEx := NewTimeoutException(streaming_types.StreamSendTimeout, remote.Server, 1500*time.Millisecond)
		t.Log(serverEx)
		test.Assert(t, serverEx != nil)
		test.Assert(t, errors.Is(serverEx, kerrors.ErrStreamingTimeout))
		test.Assert(t, errors.Is(serverEx, errStreamSendTimeout))
		test.Assert(t, serverEx.TypeId() == 12016, serverEx.TypeId())
	})
}

func Test_utilFuncs(t *testing.T) {
	// test formatCancelPath
	t.Run("formatCancelPath", func(t *testing.T) {
		// empty cancelPath
		testCp := ""
		res := formatCancelPath(testCp)
		test.Assert(t, res == "", res)
		// single node
		testCp = "A"
		res = formatCancelPath(testCp)
		test.Assert(t, res == "A", res)
		// two nodes
		testCp = "A,B"
		res = formatCancelPath(testCp)
		test.Assert(t, res == "A -> B", res)
		// three nodes with empty node name
		testCp = "A,B,"
		res = formatCancelPath(testCp)
		test.Assert(t, res == "A -> B -> ", res)
		// multiple nodes with empty node name
		testCp = "A,B,,,"
		res = formatCancelPath(testCp)
		test.Assert(t, res == "A -> B ->  ->  -> ", res)
	})
}
