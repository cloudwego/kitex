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
	"encoding/json"
	"io"

	"github.com/cloudwego/kitex/tool/cmd/kitex/discover/registry"
)

// JSONFormatter formats output as JSON
type JSONFormatter struct{}

// FormatServiceList formats service names as JSON array
func (f *JSONFormatter) FormatServiceList(services []string, w io.Writer) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(map[string]interface{}{
		"services": services,
		"count":    len(services),
	})
}

// FormatService formats service details as JSON
func (f *JSONFormatter) FormatService(service *registry.Service, w io.Writer) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(service)
}
