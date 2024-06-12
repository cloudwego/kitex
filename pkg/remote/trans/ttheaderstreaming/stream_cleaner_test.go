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
	"errors"
	"io"
	"testing"
	"time"

	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/kerrors"
)

func TestSetCleanConnectionTimeout(t *testing.T) {
	SetCleanConnectionTimeout(time.Second * 2)
	test.Assert(t, cleanConnectionTimeout == time.Second*2)

	SetCleanConnectionTimeout(time.Second)
	test.Assert(t, cleanConnectionTimeout == time.Second)
}

func Test_isRPCError(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		test.Assert(t, !isRPCError(nil))
	})

	t.Run("io.EOF", func(t *testing.T) {
		test.Assert(t, !isRPCError(io.EOF))
	})

	t.Run("kerrors.NewBizStatusError", func(t *testing.T) {
		test.Assert(t, !isRPCError(kerrors.NewBizStatusError(100, "")))
	})

	t.Run("other error", func(t *testing.T) {
		test.Assert(t, isRPCError(errors.New("test error")))
	})
}

func Test_streamCleaner(t *testing.T) {
	t.Run("rpc-error", func(t *testing.T) {
		err := streamCleaner(&mockStream{
			doRecvMsg: func(msg interface{}) error {
				return errors.New("test error")
			},
		})
		test.Assert(t, err != nil)
	})
	t.Run("non-rpc-error", func(t *testing.T) {
		err := streamCleaner(&mockStream{
			doRecvMsg: func(msg interface{}) error {
				return io.EOF
			},
		})
		test.Assert(t, err == nil)
	})
}
