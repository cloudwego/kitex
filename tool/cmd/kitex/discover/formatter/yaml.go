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
	"gopkg.in/yaml.v3"
)

// YAMLFormatter formats output as YAML
type YAMLFormatter struct{}

// FormatServiceList formats service names as YAML
func (f *YAMLFormatter) FormatServiceList(services []string, w io.Writer) error {
	data := map[string]interface{}{
		"services": services,
		"count":    len(services),
	}
	encoder := yaml.NewEncoder(w)
	defer encoder.Close()
	return encoder.Encode(data)
}

// FormatService formats service details as YAML
func (f *YAMLFormatter) FormatService(service *registry.Service, w io.Writer) error {
	encoder := yaml.NewEncoder(w)
	defer encoder.Close()
	return encoder.Encode(service)
}
