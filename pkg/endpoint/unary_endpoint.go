/*
 * Copyright 2023 CloudWeGo Authors
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

package endpoint

import "context"

type UnaryEndpoint Endpoint

type UnaryMiddleware func(next UnaryEndpoint) UnaryEndpoint

type UnaryMiddlewareBuilder func(ctx context.Context) UnaryMiddleware

func UnaryChain(mws ...UnaryMiddleware) UnaryMiddleware {
	return func(next UnaryEndpoint) UnaryEndpoint {
		for i := len(mws) - 1; i >= 0; i-- {
			next = mws[i](next)
		}
		return next
	}
}

func (mw UnaryMiddleware) ToMiddleware() Middleware {
	return func(next Endpoint) Endpoint {
		return Endpoint(mw(UnaryEndpoint(next)))
	}
}
