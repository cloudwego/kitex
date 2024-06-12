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

package ttheaderstreaming

import (
	"io"
	"time"

	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/cloudwego/kitex/pkg/remote/remotecli"
	"github.com/cloudwego/kitex/pkg/streaming"
	"github.com/cloudwego/kitex/transport"
)

var cleanConnectionTimeout = time.Second

// SetCleanConnectionTimeout sets the timeout for cleaning the connection before release.
// It's only used for client
func SetCleanConnectionTimeout(t time.Duration) {
	cleanConnectionTimeout = t
}

func init() {
	remotecli.RegisterCleaner(transport.TTHeader, &remotecli.StreamCleaner{
		Async:   true,
		Timeout: cleanConnectionTimeout,
		Clean:   streamCleaner,
	})
}

// streamCleaner discards remaining frames of the stream by reading until TrailerFrame (or an error),
// and returns the last RPC error (if any).
func streamCleaner(st streaming.Stream) error {
	st.Trailer()
	if err := st.RecvMsg(nil); isRPCError(err) {
		return err
	}
	return nil
}

func isRPCError(err error) bool {
	if err == nil || err == io.EOF {
		return false
	}
	_, isBizStatusError := err.(kerrors.BizStatusErrorIface)
	return !isBizStatusError // BizStatusError is not considered an RPC Error
}
