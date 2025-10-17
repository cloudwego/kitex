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

package shmipc

import (
	"time"

	"github.com/cloudwego/shmipc-go"
)

// Options contains all configuration options for SHMIPC transport.
// It provides fine-grained control over shared memory IPC behavior.
type Options struct {
	// Basic configuration
	Concurrency    int    // Number of sessions (concurrency), default 4
	BufferCapacity int    // Shared memory buffer capacity in bytes, default 32MB
	MemoryType     int    // Memory mapping type, default MemFd
	PathPrefix     string // Shared memory file path prefix, default "/dev/shm/shmipc_kitex"
	SliceSpec      string // Buffer slice specification, optional, JSON format

	// Extended configuration
	MaxStreamNum      int           // Maximum number of streams, default 10000
	StreamMaxIdleTime time.Duration // Maximum idle time for streams, default 5s
}

// NewDefaultOptions creates a default Options instance
func NewDefaultOptions() *Options {
	return &Options{
		Concurrency:       4,
		BufferCapacity:    32 << 20, // 32MB
		MemoryType:        int(shmipc.MemMapTypeMemFd),
		PathPrefix:        "/dev/shm/shmipc_kitex",
		MaxStreamNum:      10000,
		StreamMaxIdleTime: 5 * time.Second,
	}
}

// NewOptions creates an Options instance with custom configuration
func NewOptions(fn func(*Options)) *Options {
	opts := NewDefaultOptions()
	fn(opts)
	return opts
}

// WithConcurrency sets the number of sessions (concurrency)
func (o *Options) WithConcurrency(n int) *Options {
	if n > 0 {
		o.Concurrency = n
	}
	return o
}

// WithBufferCapacity sets the shared memory buffer capacity in bytes
func (o *Options) WithBufferCapacity(size int) *Options {
	if size > 0 {
		o.BufferCapacity = size
	}
	return o
}

// WithMemoryType sets the memory mapping type
func (o *Options) WithMemoryType(memType int) *Options {
	o.MemoryType = memType
	return o
}

// WithPathPrefix sets the shared memory file path prefix
func (o *Options) WithPathPrefix(prefix string) *Options {
	if prefix != "" {
		o.PathPrefix = prefix
	}
	return o
}

// WithSliceSpec sets the buffer slice specification
func (o *Options) WithSliceSpec(spec string) *Options {
	o.SliceSpec = spec
	return o
}

// WithMaxStreamNum sets the maximum number of streams
func (o *Options) WithMaxStreamNum(n int) *Options {
	if n > 0 {
		o.MaxStreamNum = n
	}
	return o
}

// WithStreamMaxIdleTime sets the maximum idle time for streams
func (o *Options) WithStreamMaxIdleTime(d time.Duration) *Options {
	if d > 0 {
		o.StreamMaxIdleTime = d
	}
	return o
}
