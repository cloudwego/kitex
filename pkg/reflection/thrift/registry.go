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

package thriftreflection

import (
	"sync"

	"github.com/cloudwego/thriftgo/reflection"
)

// Files is a registry for looking up or iterating over files and the
// descriptors contained within them.
type Files struct {
	filesByPath map[string]*reflection.FileDescriptor
}

func NewFiles() *Files {
	return &Files{filesByPath: map[string]*reflection.FileDescriptor{}}
}

var globalMutex sync.RWMutex

// GlobalFiles is a global registry of file descriptors.
var GlobalFiles = NewFiles()

// RegisterIDL provides function for generated code to register their reflection info to GlobalFiles.
func RegisterIDL(bytes []byte) {
	desc := reflection.Decode(bytes)
	GlobalFiles.Register(desc)
}

// Register registers the input FileDescriptor to *Files type variables.
func (f *Files) Register(desc *reflection.FileDescriptor) {
	if f == GlobalFiles {
		globalMutex.Lock()
		defer globalMutex.Unlock()
	}
	// TODO: check conflict
	f.filesByPath[desc.Filename] = desc
}

// GetFileDescriptors returns the inner registered reflection FileDescriptors.
func (f *Files) GetFileDescriptors() map[string]*reflection.FileDescriptor {
	if f == GlobalFiles {
		globalMutex.RLock()
		defer globalMutex.RUnlock()
		m := make(map[string]*reflection.FileDescriptor, len(f.filesByPath))
		for k, v := range f.filesByPath {
			m[k] = v
		}
		return m
	}
	return f.filesByPath
}
