// Copyright 2021 ByteDance Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package runtimex

import (
	_ "unsafe"
)

//go:noescape
//go:linkname runtime_procPin runtime.procPin
func runtime_procPin() int

//go:noescape
//go:linkname runtime_procUnpin runtime.procUnpin
func runtime_procUnpin()

// Pin pins current p, return pid.
// DO NOT USE if you don't know what this is.
func Pin() int {
	return runtime_procPin()
}

// Unpin unpins current p.
func Unpin() {
	runtime_procUnpin()
}

// Pid returns the id of current p.
func Pid() (id int) {
	id = runtime_procPin()
	runtime_procUnpin()
	return
}
