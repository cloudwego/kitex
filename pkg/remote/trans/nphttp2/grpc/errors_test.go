/*
 *
 * Copyright 2024 gRPC authors.
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
 *
 * This file may have been modified by CloudWeGo authors. All CloudWeGo
 * Modifications are Copyright 2021 CloudWeGo Authors.
 */

package grpc

import (
	"context"
	"errors"
	"strings"
	"testing"

	"golang.org/x/net/http2"

	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/codes"
)

var errs = []error{
	// stream error
	errHTTP2Stream,
	errClosedWithoutTrailer,
	errMiddleHeader,
	errDecodeHeader,
	errRecvDownStreamRstStream,
	errRecvUpstreamRstStream,
	errStreamDrain,
	errStreamFlowControl,
	errIllegalHeaderWrite,
	errStreamIsDone,
	errMaxStreamExceeded,
	// connection error
	errHTTP2Connection,
	errEstablishConnection,
	errHandleGoAway,
	errKeepAlive,
	errOperateHeaders,
	errNoActiveStream,
	errControlBufFinished,
	errNotReachable,
	errConnectionIsClosing,
}

func Test_err(t *testing.T) {
	for _, err := range errs {
		test.Assert(t, errors.Is(err, kerrors.ErrStreamingProtocol), err)
		test.Assert(t, kerrors.IsKitexError(err), err)
		test.Assert(t, kerrors.IsStreamingError(err), err)
		test.Assert(t, strings.Contains(err.Error(), kerrors.ErrStreamingProtocol.Error()), err)
	}
}

func Test_errMap(t *testing.T) {
	// mapping between rstCode and err
	for code, err := range rstCode2ErrMap {
		res, ok := err2RstCodeMap[err]
		test.Assert(t, ok)
		test.Assert(t, res == code)
	}
}

func Test_getMappingErrAndStatusCode(t *testing.T) {
	testcases := []struct {
		desc  string
		side  side
		input []http2.ErrCode
		want  []struct {
			err    error
			stCode codes.Code
		}
		setup func(t *testing.T)
		clean func(t *testing.T)
	}{
		{
			desc: "normal RstCode in clientSide",
			side: clientSide,
			input: []http2.ErrCode{
				http2.ErrCodeNo,
				http2.ErrCodeCancel,
			},
			want: []struct {
				err    error
				stCode codes.Code
			}{
				{
					err:    errRecvDownStreamRstStream,
					stCode: codes.Internal,
				},
				{
					err:    errRecvDownStreamRstStream,
					stCode: codes.Canceled,
				},
			},
		},
		{
			desc: "normal RstCode in serverSide",
			side: serverSide,
			input: []http2.ErrCode{
				http2.ErrCodeNo,
				http2.ErrCodeCancel,
			},
			want: []struct {
				err    error
				stCode codes.Code
			}{
				{
					err:    errRecvUpstreamRstStream,
					stCode: codes.Internal,
				},
				{
					err:    errRecvUpstreamRstStream,
					stCode: codes.Canceled,
				},
			},
		},
		{
			desc:  "custom RstCode",
			setup: customSetup,
			input: []http2.ErrCode{
				http2.ErrCode(1000),
				http2.ErrCode(1001),
				http2.ErrCode(1002),
				http2.ErrCode(1003),
				http2.ErrCode(1004),
				http2.ErrCode(1005),
				http2.ErrCode(1006),
				http2.ErrCode(1007),
				http2.ErrCode(1008),
				http2.ErrCode(1009),
				http2.ErrCode(1010),
				http2.ErrCode(1011),
				http2.ErrCode(1012),
			},
			want: []struct {
				err    error
				stCode codes.Code
			}{
				{
					err:    kerrors.ErrGracefulShutdown,
					stCode: codes.Canceled,
				},
				{
					err:    kerrors.ErrBizCanceled,
					stCode: codes.Canceled,
				},
				{
					err:    kerrors.ErrServerStreamFinished,
					stCode: codes.Canceled,
				},
				{
					err:    kerrors.ErrStreamingCanceled,
					stCode: codes.Canceled,
				},
				{
					err:    errRecvDownStreamRstStream,
					stCode: codes.Canceled,
				},
				{
					err:    errRecvUpstreamRstStream,
					stCode: codes.Canceled,
				},
				{
					err:    kerrors.ErrStreamTimeout,
					stCode: codes.Canceled,
				},
				{
					err:    errMiddleHeader,
					stCode: codes.Canceled,
				},
				{
					err:    errDecodeHeader,
					stCode: codes.Canceled,
				},
				{
					err:    errHTTP2Stream,
					stCode: codes.Canceled,
				},
				{
					err:    errClosedWithoutTrailer,
					stCode: codes.Canceled,
				},
				{
					err:    errStreamFlowControl,
					stCode: codes.Canceled,
				},
				{
					err:    kerrors.ErrMetaSizeExceeded,
					stCode: codes.Canceled,
				},
			},
			clean: customClean,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.desc, func(t *testing.T) {
			if tc.setup != nil {
				tc.setup(t)
			}
			if tc.clean != nil {
				defer tc.clean(t)
			}
			for i, rstCode := range tc.input {
				err, stCode := getMappingErrAndStatusCode(context.Background(), rstCode, tc.side)
				test.Assert(t, err == tc.want[i].err, err)
				res := stCode == tc.want[i].stCode
				if !res {
					t.Logf("stCode: %d, err: %v", stCode, err)
				}
				test.Assert(t, stCode == tc.want[i].stCode, stCode)
			}
		})
	}
}

func Test_getStatusCode(t *testing.T) {
	testcases := []struct {
		desc  string
		def   codes.Code
		input []error
		want  []codes.Code
		setup func(t *testing.T)
		clean func(t *testing.T)
	}{
		{
			desc: "without custom status code",
			def:  codes.Internal,
			input: []error{
				errHTTP2Stream,
			},
			want: []codes.Code{
				codes.Internal,
			},
		},
		{
			desc: "custom status code",
			def:  codes.Internal,
			input: []error{
				kerrors.ErrGracefulShutdown, kerrors.ErrBizCanceled, kerrors.ErrServerStreamFinished, kerrors.ErrStreamingCanceled,
				kerrors.ErrStreamTimeout, kerrors.ErrMetaSizeExceeded,
				errHTTP2Stream, errClosedWithoutTrailer, errMiddleHeader, errDecodeHeader, errRecvDownStreamRstStream, errRecvUpstreamRstStream, errStreamDrain, errStreamFlowControl, errIllegalHeaderWrite, errStreamIsDone, errMaxStreamExceeded,
				errHTTP2Connection, errEstablishConnection, errHandleGoAway, errKeepAlive, errOperateHeaders, errNoActiveStream, errControlBufFinished, errNotReachable, errConnectionIsClosing,
			},
			want: []codes.Code{
				codes.Code(200), codes.Code(201), codes.Code(202), codes.Code(203),
				codes.Code(204), codes.Code(205),
				codes.Code(10000), codes.Code(10001), codes.Code(10002), codes.Code(10003), codes.Code(10004), codes.Code(10005), codes.Code(10006), codes.Code(10007), codes.Code(10008), codes.Code(10009), codes.Code(10010),
				codes.Code(11000), codes.Code(11001), codes.Code(11002), codes.Code(11003), codes.Code(11004), codes.Code(11005), codes.Code(11006), codes.Code(11007), codes.Code(11008),
			},
			setup: customSetup,
			clean: customClean,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.desc, func(t *testing.T) {
			if tc.setup != nil {
				tc.setup(t)
			}
			if tc.clean != nil {
				defer tc.clean(t)
			}
			for i, err := range tc.input {
				code := getStatusCode(tc.def, err)
				test.Assert(t, code == tc.want[i], err)
			}
		})
	}
}

func customSetup(t *testing.T) {
	SetCustomRstCodeEnabled(true)
	SetCustomStatusCodeEnabled(true)
}

func customClean(t *testing.T) {
	SetCustomRstCodeEnabled(false)
	SetCustomStatusCodeEnabled(false)
}
