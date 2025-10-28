// Copyright 2025 CloudWeGo Authors
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

package formatter

import (
	"io"

	"github.com/cloudwego/kitex/tool/cmd/kitex/discover/registry"
)

// Formatter is the interface for output formatting
type Formatter interface {
	// FormatServiceList formats a list of service names
	FormatServiceList(services []string, w io.Writer) error

	// FormatService formats detailed service information
	FormatService(service *registry.Service, w io.Writer) error
}

// New creates a new formatter based on the format type
func New(format string) Formatter {
	switch format {
	case "json":
		return &JSONFormatter{}
	case "yaml":
		return &YAMLFormatter{}
	case "table":
		fallthrough
	default:
		return &TableFormatter{}
	}
}
