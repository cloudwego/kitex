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

package types

type CancelType uint8

const (
	LocalCascadedCancel CancelType = iota + 1
	RemoteCascadedCancel
	ActiveCancel
)

type WireError interface {
	Error() string
	StatusCode() int32
}

type CancelError interface {
	Error() string
	StatusCode() int32
	Type() CancelType
}

type CancelPoint struct {
	Name string
}

type RemoteCascadedCancelError interface {
	CancelError
	CancelPath() []CancelPoint
}

type LocalCascadedCancelError interface {
	CancelError
}

type ActiveCancelError interface {
	CancelError
}
