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
	"github.com/cloudwego/kitex/pkg/streaming"
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

	recvTmErr := newStreamRecvTimeoutException(streaming.TimeoutConfig{Timeout: 1 * time.Second})
	test.Assert(t, errors.Is(recvTmErr, kerrors.ErrStreamingTimeout), recvTmErr)
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
	errs = []error{
		newStreamRecvTimeoutException(streaming.TimeoutConfig{Timeout: 1 * time.Second}),
	}
	for _, err := range errs {
		test.Assert(t, errors.Is(err, kerrors.ErrStreamingTimeout), err)
	}
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

func Test_newStreamRecvTimeoutException(t *testing.T) {
	t.Run("DisableCancelRemote=false should not be retryable", func(t *testing.T) {
		cfg := streaming.TimeoutConfig{Timeout: 100 * time.Millisecond}
		ex := newStreamRecvTimeoutException(cfg)
		test.Assert(t, ex.TypeId() == 12014, ex.TypeId())
		test.Assert(t, ex.isSideSet())
		test.Assert(t, ex.side == clientSide, ex.side)
		test.Assert(t, strings.Contains(ex.Error(), "[client-side stream]"), ex)
		test.Assert(t, errors.Is(ex, kerrors.ErrStreamingTimeout), ex)
		test.Assert(t, !ex.isCanRetrySet(), ex)
		test.Assert(t, !checkCanRetry(ex))
		test.Assert(t, strings.Contains(ex.Error(), "stream Recv timeout"), ex)
		test.Assert(t, strings.Contains(ex.Error(), "100ms"), ex)
	})
	t.Run("DisableCancelRemote=true should be retryable", func(t *testing.T) {
		cfg := streaming.TimeoutConfig{
			Timeout:             200 * time.Millisecond,
			DisableCancelRemote: true,
		}
		ex := newStreamRecvTimeoutException(cfg)
		test.Assert(t, ex.TypeId() == 12014, ex.TypeId())
		test.Assert(t, ex.isSideSet())
		test.Assert(t, ex.side == clientSide, ex.side)
		test.Assert(t, strings.Contains(ex.Error(), "[client-side stream]"), ex)
		test.Assert(t, errors.Is(ex, kerrors.ErrStreamingTimeout), ex)
		test.Assert(t, ex.isCanRetrySet(), ex)
		test.Assert(t, checkCanRetry(ex))
		test.Assert(t, strings.Contains(ex.Error(), "DisableCancelRemote:true"), ex)
	})
	t.Run("zero timeout", func(t *testing.T) {
		cfg := streaming.TimeoutConfig{Timeout: 0}
		ex := newStreamRecvTimeoutException(cfg)
		test.Assert(t, ex.TypeId() == 12014, ex.TypeId())
		test.Assert(t, errors.Is(ex, kerrors.ErrStreamingTimeout), ex)
		test.Assert(t, !checkCanRetry(ex))
		test.Assert(t, strings.Contains(ex.Error(), "stream Recv timeout"), ex)
	})
}

func Test_newStreamTimeoutException(t *testing.T) {
	t.Run("tm > 0 returns new exception with duration", func(t *testing.T) {
		ex := newStreamTimeoutException(500 * time.Millisecond)
		test.Assert(t, ex.TypeId() == 12015, ex.TypeId())
		test.Assert(t, ex.isSideSet())
		test.Assert(t, ex.side == clientSide, ex.side)
		test.Assert(t, strings.Contains(ex.Error(), "[client-side stream]"), ex)
		test.Assert(t, errors.Is(ex, kerrors.ErrStreamingTimeout), ex)
		test.Assert(t, strings.Contains(ex.Error(), "500ms"), ex)
		test.Assert(t, strings.Contains(ex.Error(), "stream timeout, timeout in ctx"), ex)
	})
	t.Run("tm = 0 means deadline already expired", func(t *testing.T) {
		ex := newStreamTimeoutException(0)
		test.Assert(t, ex.TypeId() == 12015, ex.TypeId())
		test.Assert(t, ex.isSideSet())
		test.Assert(t, ex.side == clientSide, ex.side)
		test.Assert(t, strings.Contains(ex.Error(), "[client-side stream]"), ex)
		test.Assert(t, errors.Is(ex, kerrors.ErrStreamingTimeout), ex)
		test.Assert(t, strings.Contains(ex.Error(), "stream timeout, timeout in ctx"), ex)
	})
	t.Run("negative tm means no explicit timeout", func(t *testing.T) {
		ex := newStreamTimeoutException(notSetStreamTimeout)
		test.Assert(t, ex.TypeId() == 12015, ex.TypeId())
		test.Assert(t, errors.Is(ex, kerrors.ErrStreamingTimeout), ex)
		test.Assert(t, strings.Contains(ex.Error(), "no explicit timeout"), ex)
	})
	t.Run("multiple calls with tm > 0 return independent instances", func(t *testing.T) {
		ex1 := newStreamTimeoutException(100 * time.Millisecond)
		ex2 := newStreamTimeoutException(100 * time.Millisecond)
		test.Assert(t, ex1 != ex2, "should be different pointers")
		test.DeepEqual(t, ex1, ex2)
	})
}

func TestCanRetry(t *testing.T) {
	t.Run("withCanRetry sets the flag and returns same pointer", func(t *testing.T) {
		ex := newException("test", nil, 1000)
		test.Assert(t, !ex.isCanRetrySet())
		original := ex
		ex = ex.withCanRetry()
		test.Assert(t, ex == original, "withCanRetry should return same pointer")
		test.Assert(t, ex.isCanRetrySet())
		test.Assert(t, checkCanRetry(ex))
	})
	t.Run("newBuilder preserves canRetry", func(t *testing.T) {
		ex := newException("test", nil, 1000).withCanRetry()
		cloned := ex.newBuilder()
		test.Assert(t, cloned.isCanRetrySet())
		test.Assert(t, checkCanRetry(cloned))
		test.Assert(t, cloned != ex, "should be different pointer")
	})
}

func TestCheckCanRetry(t *testing.T) {
	t.Run("non-Exception error returns false", func(t *testing.T) {
		test.Assert(t, !checkCanRetry(errors.New("plain error")))
	})
	t.Run("nil error returns false", func(t *testing.T) {
		test.Assert(t, !checkCanRetry(nil))
	})
	t.Run("Exception without canRetry returns false", func(t *testing.T) {
		ex := newException("test", nil, 1000)
		test.Assert(t, !checkCanRetry(ex))
	})
	t.Run("Exception with canRetry returns true", func(t *testing.T) {
		ex := newException("test", nil, 1000).withCanRetry()
		test.Assert(t, checkCanRetry(ex))
	})
	t.Run("recv timeout with DisableCancelRemote=true is retryable", func(t *testing.T) {
		ex := newStreamRecvTimeoutException(streaming.TimeoutConfig{
			Timeout:             100 * time.Millisecond,
			DisableCancelRemote: true,
		})
		test.Assert(t, checkCanRetry(ex))
	})
	t.Run("recv timeout with DisableCancelRemote=false is not retryable", func(t *testing.T) {
		ex := newStreamRecvTimeoutException(streaming.TimeoutConfig{
			Timeout: 100 * time.Millisecond,
		})
		test.Assert(t, !checkCanRetry(ex))
	})
	t.Run("stream timeout is never retryable", func(t *testing.T) {
		test.Assert(t, !checkCanRetry(newStreamTimeoutException(100*time.Millisecond)))
		test.Assert(t, !checkCanRetry(newStreamTimeoutException(0)))
	})
	t.Run("wrapped Exception returns false", func(t *testing.T) {
		ex := newException("test", nil, 1000).withCanRetry()
		wrapped := fmt.Errorf("wrap: %w", ex)
		test.Assert(t, !checkCanRetry(wrapped))
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
