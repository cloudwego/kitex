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

package proto

import (
	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/dynamic"
)

type (
	ServiceDescriptor = *desc.ServiceDescriptor
	MessageDescriptor = *desc.MessageDescriptor
)

// Deprecated: this interface will be removed in v0.12.0
type Message interface {
	Marshal() ([]byte, error)
	TryGetFieldByNumber(fieldNumber int) (interface{}, error)
	TrySetFieldByNumber(fieldNumber int, val interface{}) error
}

// Deprecated: this API will be removed in v0.12.0
func NewMessage(descriptor MessageDescriptor) Message { //lint:ignore SA1019 static check
	return dynamic.NewMessage(descriptor)
}
