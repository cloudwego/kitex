// Copyright 2021 CloudWeGo Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package plugin defines the interface for implementing thriftgo plugins.
//
// A plugin is a stand-alone executable that reads input from standard input and
// writes out generated contents to the standard output while logging any warning
// messages to the standard error.
//
// When the plugin is executed by thriftgo, the input stream will be a Request object
// defined in the protocol.thrift serialized with the binary protocol. The plugin
// is expected to write a Response object to the standard output, serialized with
// the binary protocol, too. The plugin can use the exit status to indicate whether
// it finishes its jobs successfully.
//
// Refer to protocol.thrift for more information.
package plugin
