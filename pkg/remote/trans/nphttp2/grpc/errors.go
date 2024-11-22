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
	"fmt"
	"io"
	"time"

	"golang.org/x/net/http2"

	istatus "github.com/cloudwego/kitex/internal/remote/trans/grpc/status"
	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/codes"
)

// This file contains all the errors suitable for Kitex errors model.
var (
	// stream error
	errHTTP2Stream                    = fmt.Errorf("%w - %s", kerrors.ErrStreamingProtocol, "HTTP2Stream err when parsing HTTP2 frame")
	errClosedWithoutTrailer           = fmt.Errorf("%w - %s", kerrors.ErrStreamingProtocol, "client received Data frame with END_STREAM flag")
	errMiddleHeader                   = fmt.Errorf("%w - %s", kerrors.ErrStreamingProtocol, "Headers frame appeared in the middle of a stream")
	errDecodeHeader                   = fmt.Errorf("%w - %s", kerrors.ErrStreamingProtocol, "decoded Headers frame failed")
	errRecvDownStreamRstStream        = fmt.Errorf("%w - %s", kerrors.ErrStreamingProtocol, "received RstStream frame from downstream")
	errRecvUpstreamRstStream          = fmt.Errorf("%w - %s", kerrors.ErrStreamingProtocol, "receive RstStream frame fron upstream")
	errStreamDrain                    = fmt.Errorf("%w - %s", kerrors.ErrStreamingProtocol, "stream rejected by draining connection")
	errStreamFlowControl              = fmt.Errorf("%w - %s", kerrors.ErrStreamingProtocol, "stream-level flow control")
	errIllegalHeaderWrite             = fmt.Errorf("%w - %s", kerrors.ErrStreamingProtocol, "Headers frame has been already sent by server")
	errStreamIsDone                   = fmt.Errorf("%w - %s", kerrors.ErrStreamingProtocol, "stream is done")
	errMaxStreamExceeded              = fmt.Errorf("%w - %s", kerrors.ErrStreamingProtocol, "max stream exceeded")
	errRecvDownStreamGracefulShutdown = fmt.Errorf("%w - %s", kerrors.ErrStreamingProtocol, "received graceful shutdown from downstream")

	// connection error
	errHTTP2Connection     = fmt.Errorf("%w - %s", kerrors.ErrStreamingProtocol, "HTTP2Connection err when parsing HTTP2 frame")
	errEstablishConnection = fmt.Errorf("%w - %s", kerrors.ErrStreamingProtocol, "established connection failed")
	errHandleGoAway        = fmt.Errorf("%w - %s", kerrors.ErrStreamingProtocol, "handled GoAway Frame failed")
	errKeepAlive           = fmt.Errorf("%w - %s", kerrors.ErrStreamingProtocol, "keepalive failed")
	errOperateHeaders      = fmt.Errorf("%w - %s", kerrors.ErrStreamingProtocol, "operated Headers Frame failed")
	errNoActiveStream      = fmt.Errorf("%w - %s", kerrors.ErrStreamingProtocol, "no active stream")
	errControlBufFinished  = fmt.Errorf("%w - %s", kerrors.ErrStreamingProtocol, "controlbuf finished")
	errNotReachable        = fmt.Errorf("%w - %s", kerrors.ErrStreamingProtocol, "server transport is not reachable")
	errConnectionIsClosing = fmt.Errorf("%w - %s", kerrors.ErrStreamingProtocol, "connection is closing")
)

var rstCode2ErrMap = map[http2.ErrCode]error{
	http2.ErrCode(1000): kerrors.ErrGracefulShutdown,
	http2.ErrCode(1001): kerrors.ErrBizCanceled,
	http2.ErrCode(1002): kerrors.ErrServerStreamFinished,
	http2.ErrCode(1003): kerrors.ErrStreamingCanceled,
	http2.ErrCode(1004): errRecvDownStreamRstStream,
	http2.ErrCode(1005): errRecvUpstreamRstStream,
	http2.ErrCode(1006): kerrors.ErrStreamTimeout,
	http2.ErrCode(1007): errMiddleHeader,
	http2.ErrCode(1008): errDecodeHeader,
	http2.ErrCode(1009): errHTTP2Stream,
	http2.ErrCode(1010): errClosedWithoutTrailer,
	http2.ErrCode(1011): errStreamFlowControl,
	http2.ErrCode(1012): kerrors.ErrMetaSizeExceeded,
	http2.ErrCode(1013): errRecvDownStreamGracefulShutdown,
}

var err2RstCodeMap = map[error]http2.ErrCode{
	kerrors.ErrGracefulShutdown:       http2.ErrCode(1000),
	kerrors.ErrBizCanceled:            http2.ErrCode(1001),
	kerrors.ErrServerStreamFinished:   http2.ErrCode(1002),
	kerrors.ErrStreamingCanceled:      http2.ErrCode(1003),
	errRecvDownStreamRstStream:        http2.ErrCode(1004),
	errRecvUpstreamRstStream:          http2.ErrCode(1005),
	kerrors.ErrStreamTimeout:          http2.ErrCode(1006),
	errMiddleHeader:                   http2.ErrCode(1007),
	errDecodeHeader:                   http2.ErrCode(1008),
	errHTTP2Stream:                    http2.ErrCode(1009),
	errClosedWithoutTrailer:           http2.ErrCode(1010),
	errStreamFlowControl:              http2.ErrCode(1011),
	kerrors.ErrMetaSizeExceeded:       http2.ErrCode(1012),
	errRecvDownStreamGracefulShutdown: http2.ErrCode(1013),
}

// err2StatusCodeMap contains the custom status code mapping for the error.
var err2StatusCodeMap = map[error]codes.Code{
	// kitex global error
	kerrors.ErrGracefulShutdown:     codes.Code(200),
	kerrors.ErrBizCanceled:          codes.Code(201),
	kerrors.ErrServerStreamFinished: codes.Code(202),
	kerrors.ErrStreamingCanceled:    codes.Code(203),
	kerrors.ErrStreamTimeout:        codes.Code(204),
	kerrors.ErrMetaSizeExceeded:     codes.Code(205),
	// stream error
	errHTTP2Stream:                    codes.Code(10000),
	errClosedWithoutTrailer:           codes.Code(10001),
	errMiddleHeader:                   codes.Code(10002),
	errDecodeHeader:                   codes.Code(10003),
	errRecvDownStreamRstStream:        codes.Code(10004),
	errRecvUpstreamRstStream:          codes.Code(10005),
	errStreamDrain:                    codes.Code(10006),
	errStreamFlowControl:              codes.Code(10007),
	errIllegalHeaderWrite:             codes.Code(10008),
	errStreamIsDone:                   codes.Code(10009),
	errMaxStreamExceeded:              codes.Code(10010),
	errRecvDownStreamGracefulShutdown: codes.Code(10011),
	// connection error
	errHTTP2Connection:     codes.Code(11000),
	errEstablishConnection: codes.Code(11001),
	errHandleGoAway:        codes.Code(11002),
	errKeepAlive:           codes.Code(11003),
	errOperateHeaders:      codes.Code(11004),
	errNoActiveStream:      codes.Code(11005),
	errControlBufFinished:  codes.Code(11006),
	errNotReachable:        codes.Code(11007),
	errConnectionIsClosing: codes.Code(11008),
}

// crossStreamErrMap contains all the errors permitted to pass through the stream by Stream.cancel
var crossStreamErrMap = map[error]bool{
	kerrors.ErrStreamTimeout:        true,
	kerrors.ErrBizCanceled:          true,
	kerrors.ErrGracefulShutdown:     true,
	kerrors.ErrServerStreamFinished: true,
	kerrors.ErrStreamingCanceled:    true,
	errRecvUpstreamRstStream:        true,
}

var customRstCodeEnabled = false

// SetCustomRstCodeEnabled enables/disables the ErrType and custom RstCode mapping.
// it is off by default.
func SetCustomRstCodeEnabled(flag bool) {
	customRstCodeEnabled = flag
}

func isCustomRstCodeEnabled() bool {
	return customRstCodeEnabled
}

func getMappingErrAndStatusCode(ctx context.Context, rstCode http2.ErrCode, s side) (error, codes.Code) {
	mappingErr := getMappingErr(rstCode, s)
	stCode := retrieveStatusCode(ctx, rstCode, mappingErr)
	return mappingErr, stCode
}

func getMappingErr(rstCode http2.ErrCode, s side) error {
	err, ok := rstCode2ErrMap[rstCode]
	if !isCustomRstCodeEnabled() || !ok {
		if s == clientSide {
			return errRecvDownStreamRstStream
		}
		return errRecvUpstreamRstStream
	}
	return err
}

func retrieveStatusCode(ctx context.Context, errCode http2.ErrCode, mappingErr error) codes.Code {
	stCode, ok := http2ErrConvTab[errCode]
	if !ok {
		if !isCustomRstCodeEnabled() {
			klog.Warnf("transport: http2Client.handleRSTStream found no mapped gRPC status for the received nhttp2 error %v", errCode)
			stCode = codes.Unknown
		} else {
			stCode = codes.Canceled
		}
	}
	if stCode == codes.Canceled {
		if d, ok := ctx.Deadline(); ok && !d.After(time.Now()) {
			// Our deadline was already exceeded, and that was likely the cause
			// of this cancelation.  Alter the status code accordingly.
			stCode = codes.DeadlineExceeded
		}
	}
	return stCode
}

func getRstCode(err error) (rstCode http2.ErrCode) {
	if err == nil || err == io.EOF {
		return http2.ErrCodeNo
	}
	rstCode = http2.ErrCodeCancel
	statusErr, ok := err.(*istatus.Error)
	if !ok {
		return
	}
	mappingErr := istatus.GetMappingErr(statusErr)
	if mappingErr == nil {
		return
	}
	code, ok := err2RstCodeMap[mappingErr]
	if !ok {
		return
	}
	return code
}

var customStatusCodeEnabled = false

// SetCustomStatusCodeEnabled enables/disables the ErrType and custom Status code mapping.
// it is off by default.
func SetCustomStatusCodeEnabled(flag bool) {
	customStatusCodeEnabled = flag
}

func isCustomStatusCodeEnabled() bool {
	return customStatusCodeEnabled
}

func getStatusCode(def codes.Code, mappingErr error) codes.Code {
	if !isCustomStatusCodeEnabled() {
		return def
	}
	code, ok := err2StatusCodeMap[mappingErr]
	if ok {
		return code
	}
	return def
}
