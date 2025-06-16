/*
 * Copyright 2025 CloudWeGo Authors
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

package proto

import (
	"context"
	"reflect"
	"testing"

	"github.com/cloudwego/kitex/internal/test"
)

func TestWriteRaw_Write(t *testing.T) {
	ctx := context.Background()
	wr := NewRawReaderWriter()

	data := []byte("test data")
	result, err := wr.Write(ctx, data, "method", true)
	test.Assert(t, err == nil)
	resultBytes, ok := result.([]byte)
	test.Assert(t, ok)
	test.Assert(t, reflect.DeepEqual(data, resultBytes))
}

func TestReadRaw_Read(t *testing.T) {
	ctx := context.Background()
	reader := NewRawReaderWriter()

	data := []byte("test data for reading")
	result, err := reader.Read(ctx, "method", true, data)
	test.Assert(t, err == nil)

	resultBytes, ok := result.([]byte)
	test.Assert(t, ok)
	test.Assert(t, reflect.DeepEqual(data, resultBytes))
}
