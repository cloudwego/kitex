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
	"io"

	"github.com/cloudwego/netpoll"
)

type sideType int32

const (
	clientSide sideType = 1
	serverSide sideType = 2

	streamCacheSize = 32
	frameCacheSize  = 256

	connStateOpen   = 0
	connStateClosed = 1
)

func isIgnoreError(err error) bool {
	return errors.Is(err, netpoll.ErrEOF) || errors.Is(err, io.EOF) || errors.Is(err, netpoll.ErrConnClosed)
}
