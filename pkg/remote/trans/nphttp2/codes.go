/*
 * Copyright 2021 CloudWeGo Authors
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
	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/codes"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/status"
)

var (
	kitexErrConvTab = map[error]codes.Code{
		kerrors.ErrInternalException: codes.Internal,
		kerrors.ErrOverlimit:         codes.ResourceExhausted,
		kerrors.ErrRemoteOrNetwork:   codes.Unavailable,
		kerrors.ErrACL:               codes.PermissionDenied,
	}
	statusCodeConvTab = map[codes.Code]error{
		codes.Internal:          kerrors.ErrInternalException,
		codes.ResourceExhausted: kerrors.ErrOverlimit,
		codes.Unavailable:       kerrors.ErrRemoteOrNetwork,
		codes.PermissionDenied:  kerrors.ErrACL,
	}
)

func convertFromKitexToGrpc(err error) *status.Status {
	if err == nil {
		return status.New(codes.OK, "")
	}
	if c, ok := kitexErrConvTab[err]; ok {
		return status.New(c, err.Error())
	}
	return status.New(codes.Internal, err.Error())
}

func convertErrorFromGrpcToKitex(err error) error {
	s, ok := status.FromError(err)
	if !ok {
		return err
	}
	if s == nil || s.Code() == codes.OK {
		return nil
	}
	if e, ok := statusCodeConvTab[s.Code()]; ok {
		return e.(interface{ WithCause(cause error) error }).WithCause(s.Err())
	}
	return kerrors.ErrInternalException.WithCause(s.Err())
}
