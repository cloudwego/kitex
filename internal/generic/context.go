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

package generic

import (
	"context"

	"github.com/cloudwego/kitex/pkg/serviceinfo"
)

type genericStreamingKey struct{}

// WithGenericStreamingMode sets the generic streaming mode to the context.
// It provides stream mode information for the ServiceInfo.MethodInfo() lookup interface
// to obtain the correct MethodInfo in binary generic scenarios.
func WithGenericStreamingMode(ctx context.Context, sm serviceinfo.StreamingMode) context.Context {
	return context.WithValue(ctx, genericStreamingKey{}, sm)
}

// GetGenericStreamingMode gets the generic streaming mode from the context.
func GetGenericStreamingMode(ctx context.Context) serviceinfo.StreamingMode {
	sm, _ := ctx.Value(genericStreamingKey{}).(serviceinfo.StreamingMode)
	return sm
}
