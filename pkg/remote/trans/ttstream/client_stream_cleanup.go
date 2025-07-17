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

package ttstream

func (t *clientTransport) Tick() {
	var toCloseStreams []*clientStream
	t.streams.Range(func(key, value any) bool {
		s := value.(*clientStream)
		select {
		case <-s.ctx.Done():
			toCloseStreams = append(toCloseStreams, s)
		default:
		}
		return true
	})
	for _, s := range toCloseStreams {
		s.ctxDoneCallback(s.ctx)
	}
}
