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

package invoke

import (
	"net"

	"github.com/jackedelic/kitex/internal/pkg/remote/trans/invoke"
)

// Message indicates invoke message.
type Message = invoke.Message

// NewMessage creates new invoke message.
func NewMessage(local, remote net.Addr) Message {
	return invoke.NewMessage(local, remote)
}
